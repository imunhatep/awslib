package cloudcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscloudcontrol "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/cache/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func progress(status string, extra map[string]interface{}) map[string]interface{} {
	event := map[string]interface{}{
		"TypeName":        "AWS::Logs::LogGroup",
		"Identifier":      "demo",
		"RequestToken":    "token-1",
		"OperationStatus": status,
	}
	for k, v := range extra {
		event[k] = v
	}
	return map[string]interface{}{"ProgressEvent": event}
}

func describe(properties string) map[string]interface{} {
	return map[string]interface{}{
		"TypeName": "AWS::Logs::LogGroup",
		"ResourceDescription": map[string]interface{}{
			"Identifier": "demo",
			"Properties": properties,
		},
	}
}

func listOf(properties ...string) map[string]interface{} {
	descs := []map[string]interface{}{}
	for i, p := range properties {
		descs = append(descs, map[string]interface{}{"Identifier": string(rune('a' + i)), "Properties": p})
	}
	return map[string]interface{}{"TypeName": "AWS::Logs::LogGroup", "ResourceDescriptions": descs}
}

func newDataCache(t *testing.T) *cache.DataCache {
	t.Helper()
	bc, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute))
	require.NoError(t, err)
	return cache.NewDataCache().WithHandlers(handlers.NewInMemory(bc))
}

// --- 4. typed failure error -------------------------------------------------

func TestWaitReturnsRequestFailedErrorWithEvent(t *testing.T) {
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		return 200, progress("FAILED", map[string]interface{}{
			"Operation":     "CREATE",
			"ErrorCode":     "AlreadyExists",
			"StatusMessage": "already there",
		})
	})

	repo := NewCloudControlRepository(context.Background(), client)
	_, err := repo.WaitForResourceRequest("token-1", time.Minute)
	require.Error(t, err)

	var failed *RequestFailedError
	require.True(t, errors.As(err, &failed), "errors.As must reach the typed error through the go-errors wrapper")
	assert.Equal(t, cc.HandlerErrorCodeAlreadyExists, failed.Code())
	assert.Equal(t, cc.OperationStatusFailed, failed.Status())
	assert.Contains(t, failed.Error(), "CREATE AWS::Logs::LogGroup demo")
	assert.Contains(t, failed.Error(), "already there")
}

func TestWaitTreatsCancelCompleteAsFailure(t *testing.T) {
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		return 200, progress("CANCEL_COMPLETE", nil)
	})

	repo := NewCloudControlRepository(context.Background(), client)
	_, err := repo.WaitForResourceRequest("token-1", time.Minute)

	var failed *RequestFailedError
	require.True(t, errors.As(err, &failed))
	assert.Equal(t, cc.OperationStatusCancelComplete, failed.Status())
	assert.Empty(t, failed.Code())
}

// --- 5. waiter tuning -------------------------------------------------------

func TestWithPollIntervalDrivesTheWaiter(t *testing.T) {
	polls := 0
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		polls++
		if polls < 3 {
			return 200, progress("IN_PROGRESS", nil)
		}
		return 200, progress("SUCCESS", nil)
	})

	repo := NewCloudControlRepository(context.Background(), client).WithPollInterval(10*time.Millisecond, 20*time.Millisecond)

	start := time.Now()
	event, err := repo.WaitForResourceRequest("token-1", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, cc.OperationStatusSuccess, event.OperationStatus)
	assert.Equal(t, 3, polls)
	assert.Less(t, time.Since(start), 2*time.Second, "SDK default is 5s between polls; the override must apply")
}

func TestWithWaiterOptionsReplacesEarlier(t *testing.T) {
	repo := NewCloudControlRepository(context.Background(), nil).
		WithPollInterval(time.Hour, time.Hour).
		WithWaiterOptions(func(o *awscloudcontrol.ResourceRequestSuccessWaiterOptions) { o.MinDelay = time.Millisecond })

	o := awscloudcontrol.ResourceRequestSuccessWaiterOptions{}
	for _, fn := range repo.waiterOptions {
		fn(&o)
	}
	assert.Equal(t, time.Millisecond, o.MinDelay)
	assert.Zero(t, o.MaxDelay)
}

// --- 2. write-and-wait ------------------------------------------------------

func TestCreateResourceAndWaitReturnsDescribedResource(t *testing.T) {
	client, targets := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		switch target {
		case "CreateResource":
			// A freshly accepted CREATE may not know its identifier yet.
			return 200, map[string]interface{}{"ProgressEvent": map[string]interface{}{
				"RequestToken": "token-1", "OperationStatus": "IN_PROGRESS",
			}}
		case "GetResourceRequestStatus":
			return 200, progress("SUCCESS", nil)
		case "GetResource":
			assert.Equal(t, "demo", body["Identifier"])
			return 200, describe(`{"LogGroupName":"demo","RetentionInDays":7,"Tags":[{"Key":"env","Value":"prod"}]}`)
		}
		t.Fatalf("unexpected target %s", target)
		return 500, nil
	})

	repo := NewCloudControlRepository(context.Background(), client).WithPollInterval(time.Millisecond, time.Millisecond)
	resource, err := repo.CreateResourceAndWait("AWS::Logs::LogGroup", `{"LogGroupName":"demo"}`, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, resource)

	assert.Equal(t, "demo", resource.GetId())
	assert.Equal(t, "123456789012", resource.GetAccountID().String())
	assert.Equal(t, map[string]string{"env": "prod"}, resource.GetTags())
	assert.EqualValues(t, 7, resource.GetAttributes()["RetentionInDays"])
	assert.Equal(t, []string{"CreateResource", "GetResourceRequestStatus", "GetResource"}, *targets)
}

func TestUpdateResourceAndWaitSurfacesFailureWithoutDescribing(t *testing.T) {
	client, targets := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		switch target {
		case "UpdateResource":
			return 200, progress("IN_PROGRESS", nil)
		case "GetResourceRequestStatus":
			return 200, progress("FAILED", map[string]interface{}{"ErrorCode": "NotUpdatable", "StatusMessage": "createOnly"})
		}
		t.Fatalf("unexpected target %s", target)
		return 500, nil
	})

	repo := NewCloudControlRepository(context.Background(), client).WithPollInterval(time.Millisecond, time.Millisecond)
	resource, err := repo.UpdateResourceAndWait("AWS::Logs::LogGroup", "demo", `[]`, time.Minute)
	require.Error(t, err)
	assert.Nil(t, resource)

	var failed *RequestFailedError
	require.True(t, errors.As(err, &failed))
	assert.Equal(t, cc.HandlerErrorCodeNotUpdatable, failed.Code())
	assert.NotContains(t, *targets, "GetResource")
}

func TestDeleteResourceAndWaitReturnsFinalEvent(t *testing.T) {
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		if target == "DeleteResource" {
			return 200, progress("IN_PROGRESS", nil)
		}
		return 200, progress("SUCCESS", map[string]interface{}{"Operation": "DELETE"})
	})

	repo := NewCloudControlRepository(context.Background(), client).WithPollInterval(time.Millisecond, time.Millisecond)
	event, err := repo.DeleteResourceAndWait("AWS::Logs::LogGroup", "demo", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, cc.OperationDelete, event.Operation)
	assert.Equal(t, cc.OperationStatusSuccess, event.OperationStatus)
}

// --- 3. tag helpers ---------------------------------------------------------

func patchOf(t *testing.T, doc string) []map[string]interface{} {
	t.Helper()
	var patch []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(doc), &patch))
	return patch
}

func TestTagsPatchListShapeMergesByKeyKeepingOrder(t *testing.T) {
	attrs := map[string]interface{}{"Tags": []interface{}{
		map[string]interface{}{"Key": "team", "Value": "sre"},
		map[string]interface{}{"Key": "env", "Value": "dev"},
		map[string]interface{}{"Key": "gone", "Value": "x"},
	}}

	doc, err := tagsPatch("AWS::Logs::LogGroup", attrs, map[string]string{"env": "prod", "b": "2", "a": "1"}, []string{"gone"})
	require.NoError(t, err)

	patch := patchOf(t, doc)
	require.Len(t, patch, 1)
	assert.Equal(t, "add", patch[0]["op"])
	assert.Equal(t, "/Tags", patch[0]["path"])
	assert.Equal(t, []interface{}{
		map[string]interface{}{"Key": "team", "Value": "sre"},
		map[string]interface{}{"Key": "env", "Value": "prod"},
		map[string]interface{}{"Key": "a", "Value": "1"},
		map[string]interface{}{"Key": "b", "Value": "2"},
	}, patch[0]["value"], "existing order kept, replaced in place, new keys appended sorted")
}

func TestTagsPatchMapShapeFollowsCurrentValue(t *testing.T) {
	attrs := map[string]interface{}{"Tags": map[string]interface{}{"team": "sre", "gone": "x"}}

	doc, err := tagsPatch("AWS::Logs::LogGroup", attrs, map[string]string{"env": "prod"}, []string{"gone"})
	require.NoError(t, err)

	patch := patchOf(t, doc)
	assert.Equal(t, map[string]interface{}{"team": "sre", "env": "prod"}, patch[0]["value"],
		"a map-shaped Tags stays a map even for a type not in the table")
}

func TestTagsPatchUntaggedUsesTableForShape(t *testing.T) {
	doc, err := tagsPatch("AWS::SSM::Parameter", map[string]interface{}{}, map[string]string{"env": "prod"}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"env": "prod"}, patchOf(t, doc)[0]["value"])

	doc, err = tagsPatch("AWS::Logs::LogGroup", nil, map[string]string{"env": "prod"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{map[string]interface{}{"Key": "env", "Value": "prod"}}, patchOf(t, doc)[0]["value"])
}

func TestTagsPatchRejectsUnknownShape(t *testing.T) {
	_, err := tagsPatch("AWS::Logs::LogGroup", map[string]interface{}{"Tags": "oops"}, map[string]string{"a": "b"}, nil)
	require.Error(t, err)
}

func TestSetResourceTagsReadsThenPatches(t *testing.T) {
	client, targets := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		switch target {
		case "GetResource":
			return 200, describe(`{"LogGroupName":"demo","Tags":[{"Key":"team","Value":"sre"}]}`)
		case "UpdateResource":
			patch := patchOf(t, body["PatchDocument"].(string))
			assert.Equal(t, []interface{}{
				map[string]interface{}{"Key": "team", "Value": "sre"},
				map[string]interface{}{"Key": "env", "Value": "prod"},
			}, patch[0]["value"])
			return 200, progress("IN_PROGRESS", nil)
		}
		t.Fatalf("unexpected target %s", target)
		return 500, nil
	})

	repo := NewCloudControlRepository(context.Background(), client)
	event, err := repo.SetResourceTags("AWS::Logs::LogGroup", "demo", map[string]string{"env": "prod"})
	require.NoError(t, err)
	assert.Equal(t, "token-1", aws.ToString(event.RequestToken))
	assert.Equal(t, []string{"GetResource", "UpdateResource"}, *targets)

	// No-ops never touch the API.
	event, err = repo.SetResourceTags("AWS::Logs::LogGroup", "demo", nil)
	require.NoError(t, err)
	assert.Nil(t, event)
	event, err = repo.UntagResource("AWS::Logs::LogGroup", "demo")
	require.NoError(t, err)
	assert.Nil(t, event)
	assert.Len(t, *targets, 2)
}

// --- 1. cache invalidation --------------------------------------------------

func TestCachedWriteEvictsListsInItsNamespaceOnly(t *testing.T) {
	lists := 0
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		switch target {
		case "ListResources":
			lists++
			if lists == 1 {
				return 200, listOf(`{"LogGroupName":"a"}`)
			}
			return 200, listOf(`{"LogGroupName":"a"}`, `{"LogGroupName":"b"}`)
		case "CreateResource":
			return 200, progress("IN_PROGRESS", nil)
		}
		t.Fatalf("unexpected target %s", target)
		return 500, nil
	})

	dc := newDataCache(t)
	repo := NewCloudControlRepository(context.Background(), client).WithCache(dc)

	// Warm the cache: two different list shapes, both under the ListResources prefix.
	first, err := repo.ListResourcesByType("AWS::Logs::LogGroup")
	require.NoError(t, err)
	require.Len(t, first, 1)
	_, err = repo.ListResourcesByInput(&awscloudcontrol.ListResourcesInput{TypeName: aws.String("AWS::Logs::LogGroup")}, false)
	require.NoError(t, err)
	assert.Equal(t, 2, lists)

	again, err := repo.ListResourcesByType("AWS::Logs::LogGroup")
	require.NoError(t, err)
	assert.Len(t, again, 1)
	assert.Equal(t, 2, lists, "served from cache")

	// Something unrelated in the same namespace, and the same list in another namespace, must survive.
	other := dc.WithNamespace("123456789012:eu-west-1:cloudcontrol")
	require.NoError(t, other.Write(cache.Key("ListResourcesByType", "AWS::Logs::LogGroup"), []Resource{{}}))
	require.NoError(t, dc.WithNamespace("123456789012:eu-central-1:cloudcontrol").Write("Unrelated", []Resource{{}}))

	_, err = repo.CreateResource("AWS::Logs::LogGroup", `{"LogGroupName":"b"}`)
	require.NoError(t, err)

	refreshed, err := repo.ListResourcesByType("AWS::Logs::LogGroup")
	require.NoError(t, err)
	assert.Len(t, refreshed, 2, "the write evicted the cached list")
	_, err = repo.ListResourcesByInput(&awscloudcontrol.ListResourcesInput{TypeName: aws.String("AWS::Logs::LogGroup")}, false)
	require.NoError(t, err)
	assert.Equal(t, 4, lists, "both list shapes were refetched")

	var kept []Resource
	assert.True(t, other.Read(cache.Key("ListResourcesByType", "AWS::Logs::LogGroup"), &kept), "other region untouched")
	assert.True(t, dc.WithNamespace("123456789012:eu-central-1:cloudcontrol").Read("Unrelated", &kept), "other method prefix untouched")
}

func TestCachedWriteDoesNotEvictWhenRejected(t *testing.T) {
	lists := 0
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		if target == "ListResources" {
			lists++
			return 200, listOf(`{"LogGroupName":"a"}`)
		}
		return 400, map[string]interface{}{"__type": "ValidationException", "message": "bad"}
	})

	repo := NewCloudControlRepository(context.Background(), client).WithCache(newDataCache(t))
	_, err := repo.ListResourcesByType("AWS::Logs::LogGroup")
	require.NoError(t, err)

	_, err = repo.DeleteResource("AWS::Logs::LogGroup", "a")
	require.Error(t, err)

	_, err = repo.ListResourcesByType("AWS::Logs::LogGroup")
	require.NoError(t, err)
	assert.Equal(t, 1, lists, "a rejected write changed nothing, so the cache stays")
}

func TestCachedAndWaitEvictsAfterCompletion(t *testing.T) {
	lists := 0
	client, _ := stubCloudControl(t, func(target string, body map[string]interface{}) (int, interface{}) {
		switch target {
		case "ListResources":
			lists++
			return 200, listOf(`{"LogGroupName":"a"}`)
		case "DeleteResource":
			return 200, progress("IN_PROGRESS", nil)
		case "GetResourceRequestStatus":
			return 200, progress("SUCCESS", nil)
		}
		t.Fatalf("unexpected target %s", target)
		return 500, nil
	})

	repo := NewCloudControlRepository(context.Background(), client).
		WithPollInterval(time.Millisecond, time.Millisecond).
		WithCache(newDataCache(t))

	_, err := repo.ListResourcesByType("AWS::Logs::LogGroup")
	require.NoError(t, err)

	event, err := repo.DeleteResourceAndWait("AWS::Logs::LogGroup", "a", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, cc.OperationStatusSuccess, event.OperationStatus)

	_, err = repo.ListResourcesByType("AWS::Logs::LogGroup")
	require.NoError(t, err)
	assert.Equal(t, 2, lists)
	assert.Same(t, repo.Repository(), repo.repo)
}
