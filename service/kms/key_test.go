package kms

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
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

func keyArn(id string) string {
	return "arn:aws:kms:" + testRegion + ":" + testAccount + ":key/" + id
}

func TestNewKey(t *testing.T) {
	created := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	key := NewKey(fakeClient{}, kmstypes.KeyMetadata{
		KeyId:        aws.String("k1"),
		Arn:          aws.String(keyArn("k1")),
		CreationDate: aws.Time(created),
		KeyManager:   kmstypes.KeyManagerTypeCustomer,
		KeyState:     kmstypes.KeyStateEnabled,
		KeyUsage:     kmstypes.KeyUsageTypeEncryptDecrypt,
		KeySpec:      kmstypes.KeySpecSymmetricDefault,
		Origin:       kmstypes.OriginTypeAwsKms,
		Enabled:      true,
		Description:  aws.String("app data"),
	}, []string{"alias/zeta", "alias/app"}, map[string]string{"Team": "platform"})

	assert.Equal(t, "k1", key.GetId())
	assert.Equal(t, keyArn("k1"), key.GetArn())
	assert.Equal(t, cfg.ResourceTypeKey, key.GetType())
	assert.Equal(t, created, key.GetCreatedAt())
	assert.Equal(t, ptypes.AwsAccountID(testAccount), key.GetAccountID())

	assert.Equal(t, []string{"alias/app", "alias/zeta"}, key.GetAliases(), "aliases are sorted")
	assert.Equal(t, "app", key.GetName(), "first sorted alias, without the alias/ prefix")

	assert.True(t, key.IsDescribed())
	assert.True(t, key.TagsKnown())
	assert.True(t, key.IsCustomerManaged())
	assert.False(t, key.IsAwsManaged())
	assert.True(t, key.IsEnabled())
	assert.False(t, key.IsMultiRegion())
	assert.False(t, key.IsPendingDeletion())
	assert.Empty(t, key.GetMultiRegionKeyType())
	assert.Nil(t, key.GetDeletionDate())
	assert.Equal(t, "CUSTOMER", key.GetKeyManager())
	assert.Equal(t, "Enabled", key.GetKeyState())
	assert.Equal(t, "ENCRYPT_DECRYPT", key.GetKeyUsage())
	assert.Equal(t, "SYMMETRIC_DEFAULT", key.GetKeySpec())
	assert.Equal(t, "AWS_KMS", key.GetOrigin())
	assert.Equal(t, "app data", key.GetDescription())
	assert.Equal(t, "platform", key.GetTagValue("Team"))
}

func TestKeyNameFallsBackToKeyID(t *testing.T) {
	key := NewKey(fakeClient{}, kmstypes.KeyMetadata{KeyId: aws.String("k1")}, nil, nil)

	assert.Equal(t, "k1", key.GetName())
	assert.Nil(t, key.GetAliases())
}

func TestKeyAwsManagedAliasName(t *testing.T) {
	key := NewKey(fakeClient{}, kmstypes.KeyMetadata{
		KeyId:      aws.String("k2"),
		KeyManager: kmstypes.KeyManagerTypeAws,
	}, []string{"alias/aws/s3"}, nil)

	assert.Equal(t, "aws/s3", key.GetName())
	assert.True(t, key.IsAwsManaged())
	assert.True(t, key.TagsKnown(), "AWS managed keys have no tags to read")
	assert.Empty(t, key.GetTags())
}

func TestKeyPendingDeletionAndMultiRegion(t *testing.T) {
	deletion := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	key := NewKey(fakeClient{}, kmstypes.KeyMetadata{
		KeyId:        aws.String("k1"),
		KeyState:     kmstypes.KeyStatePendingDeletion,
		DeletionDate: aws.Time(deletion),
		MultiRegion:  aws.Bool(true),
		MultiRegionConfiguration: &kmstypes.MultiRegionConfiguration{
			MultiRegionKeyType: kmstypes.MultiRegionKeyTypePrimary,
		},
	}, nil, nil)

	assert.True(t, key.IsPendingDeletion())
	assert.Equal(t, deletion, *key.GetDeletionDate())
	assert.True(t, key.IsMultiRegion())
	assert.Equal(t, "PRIMARY", key.GetMultiRegionKeyType())

	key.KeyState = kmstypes.KeyStatePendingReplicaDeletion
	assert.True(t, key.IsPendingDeletion())
}

func TestNewUndescribedKey(t *testing.T) {
	key := NewUndescribedKey(fakeClient{}, kmstypes.KeyListEntry{
		KeyId:  aws.String("k3"),
		KeyArn: aws.String(keyArn("k3")),
	}, []string{"alias/locked"}, assert.AnError)

	assert.Equal(t, "k3", key.GetId())
	assert.Equal(t, keyArn("k3"), key.GetArn())
	assert.Equal(t, "k3", aws.ToString(key.KeyId))
	assert.Equal(t, keyArn("k3"), aws.ToString(key.Arn))
	assert.Equal(t, "locked", key.GetName())

	assert.False(t, key.IsDescribed())
	assert.Equal(t, assert.AnError.Error(), key.DescribeError)
	assert.False(t, key.TagsKnown(), "an undescribed key's tags are unknown, not empty")
	assert.NotEmpty(t, key.TagsError)
	assert.Empty(t, key.GetKeyManager())
	assert.Empty(t, key.GetKeyState())
}

func TestKeyGetTagsAndAliasesReturnCopies(t *testing.T) {
	key := NewKey(fakeClient{}, kmstypes.KeyMetadata{KeyId: aws.String("k1")}, []string{"alias/a"}, map[string]string{"Team": "platform"})

	tags := key.GetTags()
	tags["Team"] = "changed"
	assert.Equal(t, "platform", key.GetTags()["Team"])

	aliases := key.GetAliases()
	aliases[0] = "alias/changed"
	assert.Equal(t, "alias/a", key.GetAliases()[0])
}

func TestKeyGobRoundTrip(t *testing.T) {
	key := NewKey(fakeClient{}, kmstypes.KeyMetadata{
		KeyId:                       aws.String("k1"),
		Arn:                         aws.String(keyArn("k1")),
		CreationDate:                aws.Time(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
		KeyManager:                  kmstypes.KeyManagerTypeCustomer,
		EncryptionAlgorithms:        []kmstypes.EncryptionAlgorithmSpec{kmstypes.EncryptionAlgorithmSpecSymmetricDefault},
		MultiRegion:                 aws.Bool(true),
		PendingDeletionWindowInDays: aws.Int32(7),
		MultiRegionConfiguration: &kmstypes.MultiRegionConfiguration{
			MultiRegionKeyType: kmstypes.MultiRegionKeyTypePrimary,
			PrimaryKey:         &kmstypes.MultiRegionKey{Arn: aws.String(keyArn("k1")), Region: aws.String(testRegion)},
			ReplicaKeys:        []kmstypes.MultiRegionKey{{Arn: aws.String(keyArn("k1")), Region: aws.String("us-east-1")}},
		},
		XksKeyConfiguration: &kmstypes.XksKeyConfigurationType{Id: aws.String("xks")},
	}, []string{"alias/app"}, map[string]string{"Team": "platform"})
	key.TagsError = "denied"

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode([]Key{key}))

	var decoded []Key
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))
	require.Len(t, decoded, 1)

	assert.Equal(t, key.GetArn(), decoded[0].GetArn())
	assert.Equal(t, "app", decoded[0].GetName())
	assert.Equal(t, "platform", decoded[0].GetTagValue("Team"))
	assert.Equal(t, "denied", decoded[0].TagsError)
	assert.Equal(t, "PRIMARY", decoded[0].GetMultiRegionKeyType())
	assert.Equal(t, "xks", aws.ToString(decoded[0].XksKeyConfiguration.Id))
}

// kmsCall is one request the stub received.
type kmsCall struct {
	op   string
	body map[string]any
}

// stubKMS dispatches KMS JSON calls by X-Amz-Target to handle.
func stubKMS(t *testing.T, handle func(call kmsCall) (int, any)) (*v3.Client, func() []kmsCall) {
	t.Helper()

	var (
		mu    sync.Mutex
		calls []kmsCall
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)

		call := kmsCall{op: strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "TrentService.")}
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

	return client, func() []kmsCall {
		mu.Lock()
		defer mu.Unlock()

		return append([]kmsCall(nil), calls...)
	}
}

func apiError(code string) map[string]any {
	return map[string]any{"__type": code, "message": code + " for test"}
}

func metadata(id string, manager kmstypes.KeyManagerType) map[string]any {
	return map[string]any{
		"KeyMetadata": map[string]any{
			"KeyId":      id,
			"Arn":        keyArn(id),
			"KeyManager": string(manager),
			"KeyState":   "Enabled",
			"Enabled":    true,
		},
	}
}

func TestListKeysAll(t *testing.T) {
	client, calls := stubKMS(t, func(call kmsCall) (int, any) {
		switch call.op {
		case "ListKeys":
			if call.body["Marker"] == nil {
				return 200, map[string]any{
					"Keys": []map[string]any{
						{"KeyId": "k1", "KeyArn": keyArn("k1")},
						{"KeyId": "k2", "KeyArn": keyArn("k2")},
						{"KeyId": "k3", "KeyArn": keyArn("k3")},
					},
					"Truncated":  true,
					"NextMarker": "page2",
				}
			}

			return 200, map[string]any{
				"Keys": []map[string]any{
					{"KeyId": "k4", "KeyArn": keyArn("k4")},
					{"KeyId": "k5", "KeyArn": keyArn("k5")},
				},
			}

		case "ListAliases":
			return 200, map[string]any{
				"Aliases": []map[string]any{
					{"AliasName": "alias/zeta", "TargetKeyId": "k1"},
					{"AliasName": "alias/app", "TargetKeyId": "k1"},
					{"AliasName": "alias/aws/s3", "TargetKeyId": "k2"},
					{"AliasName": "alias/locked", "TargetKeyId": "k3"},
					{"AliasName": "alias/aws/unused"},
				},
			}

		case "DescribeKey":
			switch call.body["KeyId"] {
			case "k1", "k4":
				return 200, metadata(call.body["KeyId"].(string), kmstypes.KeyManagerTypeCustomer)
			case "k2":
				return 200, metadata("k2", kmstypes.KeyManagerTypeAws)
			case "k3":
				return 400, apiError("AccessDeniedException")
			case "k5":
				return 400, apiError("NotFoundException")
			}

		case "ListResourceTags":
			switch call.body["KeyId"] {
			case "k1":
				return 200, map[string]any{"Tags": []map[string]any{{"TagKey": "Team", "TagValue": "platform"}}}
			case "k4":
				return 400, apiError("AccessDeniedException")
			}
		}

		t.Errorf("unexpected call %s %v", call.op, call.body)

		return 500, apiError("InternalException")
	})

	keys, err := NewKmsRepository(context.Background(), client).ListKeysAll()
	require.NoError(t, err)

	byID := map[string]Key{}
	for _, key := range keys {
		byID[key.GetId()] = key
	}

	require.Len(t, byID, 4, "k5 was deleted during the listing and is dropped")

	k1 := byID["k1"]
	assert.Equal(t, "app", k1.GetName())
	assert.Equal(t, []string{"alias/app", "alias/zeta"}, k1.GetAliases())
	assert.Equal(t, map[string]string{"Team": "platform"}, k1.GetTags())
	assert.True(t, k1.IsDescribed())
	assert.True(t, k1.TagsKnown())

	k2 := byID["k2"]
	assert.Equal(t, "aws/s3", k2.GetName())
	assert.True(t, k2.IsAwsManaged())
	assert.True(t, k2.TagsKnown())

	k3 := byID["k3"]
	assert.False(t, k3.IsDescribed(), "a key policy denial keeps the key")
	assert.Contains(t, k3.DescribeError, "AccessDeniedException")
	assert.False(t, k3.TagsKnown())
	assert.Equal(t, keyArn("k3"), k3.GetArn())
	assert.Equal(t, "locked", k3.GetName())

	k4 := byID["k4"]
	assert.True(t, k4.IsDescribed())
	assert.False(t, k4.TagsKnown(), "an unreadable tag set is not an empty one")
	assert.Contains(t, k4.TagsError, "AccessDeniedException")
	assert.Empty(t, k4.GetTags())

	var tagged []string
	for _, call := range calls() {
		if call.op == "ListResourceTags" {
			tagged = append(tagged, call.body["KeyId"].(string))
		}
	}
	sort.Strings(tagged)
	assert.Equal(t, []string{"k1", "k4"}, tagged, "tags are read for described customer managed keys only")
}

func TestListKeysAllFailsWhenAliasesFail(t *testing.T) {
	client, _ := stubKMS(t, func(call kmsCall) (int, any) {
		switch call.op {
		case "ListKeys":
			return 200, map[string]any{"Keys": []map[string]any{{"KeyId": "k1", "KeyArn": keyArn("k1")}}}
		case "ListAliases":
			return 400, apiError("AccessDeniedException")
		}

		t.Errorf("unexpected call %s", call.op)

		return 500, apiError("InternalException")
	})

	keys, err := NewKmsRepository(context.Background(), client).ListKeysAll()
	require.Error(t, err)
	assert.Nil(t, keys)
}

func TestListKeysAllFailsWhenListKeysFails(t *testing.T) {
	client, calls := stubKMS(t, func(call kmsCall) (int, any) {
		return 400, apiError("AccessDeniedException")
	})

	keys, err := NewKmsRepository(context.Background(), client).ListKeysAll()
	require.Error(t, err)
	assert.Nil(t, keys)
	assert.Len(t, calls(), 1)
}

func TestListKeysAllEmptyRegion(t *testing.T) {
	client, calls := stubKMS(t, func(call kmsCall) (int, any) {
		return 200, map[string]any{"Keys": []map[string]any{}}
	})

	keys, err := NewKmsRepository(context.Background(), client).ListKeysAll()
	require.NoError(t, err)
	assert.Empty(t, keys)
	assert.Len(t, calls(), 1, "no aliases sweep for a region without keys")
}

func TestClassifyErr(t *testing.T) {
	assert.NoError(t, classifyErr(nil))
	assert.ErrorIs(t, classifyErr(&kmstypes.NotFoundException{Message: aws.String("gone")}), ErrNotFound)
	assert.NotErrorIs(t, classifyErr(assert.AnError), ErrNotFound)
}
