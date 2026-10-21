package cloudcontrol

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const stsIdentity = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123456789012:user/test</Arn>
    <UserId>AIDATEST</UserId>
    <Account>123456789012</Account>
  </GetCallerIdentityResult>
</GetCallerIdentityResponse>`

// stubCloudControl serves STS GetCallerIdentity (so v3.NewClient can resolve the
// account) and dispatches Cloud Control calls by their X-Amz-Target header to
// the given handler, which receives the decoded request body and returns the
// JSON response document.
func stubCloudControl(t *testing.T, handle func(target string, body map[string]interface{}) (int, interface{})) (*v3.Client, *[]string) {
	t.Helper()

	targets := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)

		target := r.Header.Get("X-Amz-Target")
		if target == "" {
			// STS query protocol
			require.Contains(t, string(raw), "Action=GetCallerIdentity")
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(stsIdentity))
			return
		}

		targets = append(targets, strings.TrimPrefix(target, "CloudApiService."))

		body := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(raw, &body))

		status, resp := handle(strings.TrimPrefix(target, "CloudApiService."), body)
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.WriteHeader(status)
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)

	client, err := v3.NewClient(context.Background(),
		config.WithRegion("eu-central-1"),
		config.WithBaseEndpoint(srv.URL),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithRetryMaxAttempts(1),
	)
	require.NoError(t, err)

	return client, &targets
}

func TestCreateResourceReturnsProgressEvent(t *testing.T) {
	client, targets := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		assert.Equal(t, "CreateResource", target)
		assert.Equal(t, "AWS::Logs::LogGroup", body["TypeName"])
		assert.Equal(t, `{"LogGroupName":"demo"}`, body["DesiredState"])
		assert.NotEmpty(t, body["ClientToken"], "SDK idempotency middleware fills ClientToken")

		return 200, map[string]interface{}{
			"ProgressEvent": map[string]interface{}{
				"TypeName":        "AWS::Logs::LogGroup",
				"Identifier":      "demo",
				"RequestToken":    "token-1",
				"Operation":       "CREATE",
				"OperationStatus": "IN_PROGRESS",
			},
		}
	})

	repo := NewCloudControlRepository(context.Background(), client)
	event, err := repo.CreateResource("AWS::Logs::LogGroup", `{"LogGroupName":"demo"}`)
	require.NoError(t, err)
	require.NotNil(t, event)

	assert.Equal(t, "token-1", aws.ToString(event.RequestToken))
	assert.Equal(t, cc.OperationStatusInProgress, event.OperationStatus)
	assert.Equal(t, []string{"CreateResource"}, *targets)
}

func TestUpdateAndDeleteResourceSendIdentifier(t *testing.T) {
	client, targets := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		assert.Equal(t, "AWS::Logs::LogGroup", body["TypeName"])
		assert.Equal(t, "demo", body["Identifier"])
		if target == "UpdateResource" {
			assert.Equal(t, `[{"op":"replace","path":"/RetentionInDays","value":7}]`, body["PatchDocument"])
		}

		return 200, map[string]interface{}{
			"ProgressEvent": map[string]interface{}{
				"Identifier":      "demo",
				"RequestToken":    "token-" + target,
				"OperationStatus": "PENDING",
			},
		}
	})

	repo := NewCloudControlRepository(context.Background(), client)

	event, err := repo.UpdateResource("AWS::Logs::LogGroup", "demo", `[{"op":"replace","path":"/RetentionInDays","value":7}]`)
	require.NoError(t, err)
	assert.Equal(t, "token-UpdateResource", aws.ToString(event.RequestToken))

	event, err = repo.DeleteResource("AWS::Logs::LogGroup", "demo")
	require.NoError(t, err)
	assert.Equal(t, "token-DeleteResource", aws.ToString(event.RequestToken))

	assert.Equal(t, []string{"UpdateResource", "DeleteResource"}, *targets)
}

func TestWriteErrorsAreReturnedNotSwallowed(t *testing.T) {
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		return 400, map[string]interface{}{
			"__type":  "AlreadyExistsException",
			"message": "Resource already exists",
		}
	})

	repo := NewCloudControlRepository(context.Background(), client)
	event, err := repo.CreateResource("AWS::Logs::LogGroup", `{}`)
	require.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "AlreadyExists")
}

// A FAILED request has to come back with the handler's ErrorCode and
// StatusMessage: the SDK waiter's own error carries neither, and they are the
// only thing a caller can act on.
func TestWaitForResourceRequestSurfacesFailure(t *testing.T) {
	polls := 0
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		assert.Equal(t, "GetResourceRequestStatus", target)
		assert.Equal(t, "token-1", body["RequestToken"])
		polls++

		return 200, map[string]interface{}{
			"ProgressEvent": map[string]interface{}{
				"RequestToken":    "token-1",
				"Operation":       "DELETE",
				"OperationStatus": "FAILED",
				"ErrorCode":       "NotFound",
				"StatusMessage":   "Resource of type 'AWS::Logs::LogGroup' with identifier 'demo' was not found.",
			},
		}
	})

	repo := NewCloudControlRepository(context.Background(), client)
	event, err := repo.WaitForResourceRequest("token-1", time.Minute)
	require.Error(t, err)
	require.NotNil(t, event)

	assert.Equal(t, cc.OperationStatusFailed, event.OperationStatus)
	assert.Equal(t, cc.HandlerErrorCodeNotFound, event.ErrorCode)
	assert.Contains(t, err.Error(), "NotFound")
	assert.Contains(t, err.Error(), "was not found")
	assert.Equal(t, 2, polls, "one waiter poll, then one status fetch for the message")
}

func TestWaitForResourceRequestReturnsSuccessEvent(t *testing.T) {
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		return 200, map[string]interface{}{
			"ProgressEvent": map[string]interface{}{
				"RequestToken":    "token-1",
				"Identifier":      "demo",
				"OperationStatus": "SUCCESS",
			},
		}
	})

	repo := NewCloudControlRepository(context.Background(), client)
	event, err := repo.WaitForResourceRequest("token-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, cc.OperationStatusSuccess, event.OperationStatus)
	assert.Equal(t, "demo", aws.ToString(event.Identifier))
}

func TestFindResourceRequestsPaginatesAndFilters(t *testing.T) {
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		assert.Equal(t, "ListResourceRequests", target)

		filter, ok := body["ResourceRequestStatusFilter"].(map[string]interface{})
		require.True(t, ok, "status filter is sent")
		assert.Equal(t, []interface{}{"FAILED"}, filter["OperationStatuses"])

		if body["NextToken"] == nil {
			return 200, map[string]interface{}{
				"NextToken": "page-2",
				"ResourceRequestStatusSummaries": []map[string]interface{}{
					{"RequestToken": "a", "OperationStatus": "FAILED"},
				},
			}
		}

		return 200, map[string]interface{}{
			"ResourceRequestStatusSummaries": []map[string]interface{}{
				{"RequestToken": "b", "OperationStatus": "FAILED"},
			},
		}
	})

	repo := NewCloudControlRepository(context.Background(), client)
	requests, err := repo.FindResourceRequests(cc.OperationStatusFailed)
	require.NoError(t, err)
	require.Len(t, requests, 2)
	assert.Equal(t, "a", aws.ToString(requests[0].RequestToken))
	assert.Equal(t, "b", aws.ToString(requests[1].RequestToken))
}

// The generated cached wrapper must not pick up any write on its own: the
// write methods it has come from the hand-written write_cached.go, which
// evicts on success. A generated write would be cached, never re-sent.
func TestGeneratedCachedFileHasNoWrites(t *testing.T) {
	src, err := os.ReadFile("cached_repository.go")
	require.NoError(t, err)

	for _, name := range []string{"CreateResource", "UpdateResource", "DeleteResource", "CancelResourceRequest", "SetResourceTags", "UntagResource", "WaitForResourceRequest", "DescribeResourceRequest", "FindResourceRequests"} {
		assert.NotContains(t, string(src), "func (c *CloudControlRepositoryCached) "+name+"(", name)
	}
}
