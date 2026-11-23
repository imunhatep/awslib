package ecr

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAccount = "123456789012"
	testRegion  = "eu-central-1"
)

type fakeClient struct{}

func (fakeClient) GetRegion() ptypes.AwsRegion       { return testRegion }
func (fakeClient) GetAccountID() ptypes.AwsAccountID { return testAccount }

func repoArn(name string) string {
	return "arn:aws:ecr:" + testRegion + ":" + testAccount + ":repository/" + name
}

func TestNewRepo(t *testing.T) {
	created := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	repo := NewRepo(fakeClient{}, ecrtypes.Repository{
		RepositoryName:             aws.String("team/app"),
		RepositoryArn:              aws.String(repoArn("team/app")),
		RepositoryUri:              aws.String(testAccount + ".dkr.ecr." + testRegion + ".amazonaws.com/team/app"),
		RegistryId:                 aws.String(testAccount),
		CreatedAt:                  aws.Time(created),
		ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutableWithExclusion,
		ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
		EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
			EncryptionType: ecrtypes.EncryptionTypeKms,
			KmsKey:         aws.String("arn:aws:kms:" + testRegion + ":" + testAccount + ":key/k1"),
		},
	}, map[string]string{"Team": "platform"})

	assert.Equal(t, "team/app", repo.GetId())
	assert.Equal(t, "team/app", repo.GetName())
	assert.Equal(t, repoArn("team/app"), repo.GetArn())
	assert.Equal(t, cfg.ResourceTypeECRRepository, repo.GetType())
	assert.Equal(t, created, repo.GetCreatedAt())
	assert.Equal(t, testAccount+".dkr.ecr."+testRegion+".amazonaws.com/team/app", repo.GetUri())
	assert.Equal(t, testAccount, repo.GetRegistryId())
	assert.Equal(t, "IMMUTABLE_WITH_EXCLUSION", repo.GetTagMutability())
	assert.True(t, repo.IsTagImmutable())
	assert.True(t, repo.IsScanOnPush())
	assert.Equal(t, "KMS", repo.GetEncryptionType())
	assert.Equal(t, "arn:aws:kms:"+testRegion+":"+testAccount+":key/k1", repo.GetKmsKey())
	assert.Equal(t, "platform", repo.GetTagValue("Team"))
}

func TestRepoDefaults(t *testing.T) {
	repo := NewRepo(fakeClient{}, ecrtypes.Repository{
		RepositoryName:     aws.String("app"),
		ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
	}, nil)

	assert.Empty(t, repo.GetArn())
	assert.False(t, repo.IsTagImmutable())
	assert.False(t, repo.IsScanOnPush())
	assert.Equal(t, "AES256", repo.GetEncryptionType(), "ECR encrypts with AES256 unless configured otherwise")
	assert.Empty(t, repo.GetKmsKey())
	assert.Empty(t, repo.GetTags())
}

func TestRepoGetTagsReturnsCopy(t *testing.T) {
	repo := NewRepo(fakeClient{}, ecrtypes.Repository{RepositoryName: aws.String("app")}, map[string]string{"Team": "platform"})

	tags := repo.GetTags()
	tags["Team"] = "changed"
	assert.Equal(t, "platform", repo.GetTags()["Team"])
}

func TestRepoGobRoundTrip(t *testing.T) {
	repo := NewRepo(fakeClient{}, ecrtypes.Repository{
		RepositoryName:             aws.String("app"),
		RepositoryArn:              aws.String(repoArn("app")),
		CreatedAt:                  aws.Time(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
		ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
		EncryptionConfiguration:    &ecrtypes.EncryptionConfiguration{EncryptionType: ecrtypes.EncryptionTypeAes256},
		ImageTagMutabilityExclusionFilters: []ecrtypes.ImageTagMutabilityExclusionFilter{
			{Filter: aws.String("latest"), FilterType: ecrtypes.ImageTagMutabilityExclusionFilterTypeWildcard},
		},
	}, map[string]string{"Team": "platform"})

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode([]Repo{repo}))

	var decoded []Repo
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))
	require.Len(t, decoded, 1)

	assert.Equal(t, repo.GetArn(), decoded[0].GetArn())
	assert.True(t, decoded[0].IsScanOnPush())
	assert.Equal(t, "platform", decoded[0].GetTagValue("Team"))
	assert.Equal(t, "latest", aws.ToString(decoded[0].ImageTagMutabilityExclusionFilters[0].Filter))
}

// ecrCall is one request the stub received.
type ecrCall struct {
	op   string
	body map[string]any
}

// stubECR dispatches ECR JSON calls by X-Amz-Target to handle.
func stubECR(t *testing.T, handle func(call ecrCall) (int, any)) (*v3.Client, func() []ecrCall) {
	t.Helper()

	var (
		mu    sync.Mutex
		calls []ecrCall
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)

		call := ecrCall{op: strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "AmazonEC2ContainerRegistry_V20150921.")}
		require.NoError(t, json.Unmarshal(raw, &call.body))

		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()

		status, resp := handle(call)
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(status)
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)

	client, err := v3.NewClientForAccount(context.Background(), testAccount,
		config.WithRegion(testRegion),
		config.WithBaseEndpoint(srv.URL),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithRetryMaxAttempts(1),
	)
	require.NoError(t, err)

	return client, func() []ecrCall {
		mu.Lock()
		defer mu.Unlock()

		return append([]ecrCall(nil), calls...)
	}
}

func apiError(code string) map[string]any {
	return map[string]any{"__type": code, "message": code + " for test"}
}

func describeResponse(names ...string) map[string]any {
	repos := make([]map[string]any, 0, len(names))
	for _, name := range names {
		repos = append(repos, map[string]any{
			"repositoryName": name,
			"repositoryArn":  repoArn(name),
			"registryId":     testAccount,
		})
	}

	return map[string]any{"repositories": repos}
}

func TestListRepositoriesAll(t *testing.T) {
	client, calls := stubECR(t, func(call ecrCall) (int, any) {
		switch call.op {
		case "DescribeRepositories":
			if call.body["nextToken"] == nil {
				resp := describeResponse("app", "gone")
				resp["nextToken"] = "page2"

				return 200, resp
			}

			return 200, describeResponse("base")

		case "ListTagsForResource":
			switch call.body["resourceArn"] {
			case repoArn("app"):
				return 200, map[string]any{"tags": []map[string]any{{"Key": "Team", "Value": "platform"}}}
			case repoArn("base"):
				return 200, map[string]any{"tags": []map[string]any{}}
			case repoArn("gone"):
				return 400, apiError("RepositoryNotFoundException")
			}
		}

		t.Errorf("unexpected call %s %v", call.op, call.body)

		return 500, apiError("ServerException")
	})

	repos, err := NewEcrRepository(context.Background(), client).ListRepositoriesAll()
	require.NoError(t, err)

	require.Len(t, repos, 2, "a repository deleted during the listing is dropped")
	assert.Equal(t, "app", repos[0].GetName())
	assert.Equal(t, map[string]string{"Team": "platform"}, repos[0].GetTags())
	assert.Equal(t, "base", repos[1].GetName())
	assert.Empty(t, repos[1].GetTags())

	assert.Len(t, calls(), 5)
}

func TestListRepositoriesAllFailsOnTagReadFailure(t *testing.T) {
	client, _ := stubECR(t, func(call ecrCall) (int, any) {
		switch call.op {
		case "DescribeRepositories":
			return 200, describeResponse("app", "locked")
		case "ListTagsForResource":
			if call.body["resourceArn"] == repoArn("locked") {
				return 400, apiError("AccessDeniedException")
			}

			return 200, map[string]any{"tags": []map[string]any{}}
		}

		return 500, apiError("ServerException")
	})

	repos, err := NewEcrRepository(context.Background(), client).ListRepositoriesAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), repoArn("locked"))
	assert.Nil(t, repos)
}

func TestListRepositoriesAllFailsOnDescribeFailure(t *testing.T) {
	client, _ := stubECR(t, func(call ecrCall) (int, any) {
		return 400, apiError("AccessDeniedException")
	})

	repos, err := NewEcrRepository(context.Background(), client).ListRepositoriesAll()
	require.Error(t, err)
	assert.Nil(t, repos)
}

func TestClassifyErr(t *testing.T) {
	assert.NoError(t, classifyErr(nil))
	assert.ErrorIs(t, classifyErr(&ecrtypes.RepositoryNotFoundException{Message: aws.String("gone")}), ErrNotFound)
	assert.NotErrorIs(t, classifyErr(assert.AnError), ErrNotFound)
}
