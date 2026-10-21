package cloudcontrol

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscloudcontrol "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// Cloud Control writes are asynchronous: every mutating call returns a
// ProgressEvent carrying a RequestToken, and the resource reaches its final
// state only once DescribeResourceRequest (or WaitForResourceRequest) reports
// OperationStatusSuccess. A nil error from CreateResource means the request was
// accepted, not that the resource exists.
//
// None of the methods in this file are picked up by generate-cached: their names
// deliberately avoid the Get*/List* prefixes, because a request status changes
// between two calls and a cached write would never be re-sent. Cached read
// results for the type are not invalidated either; a consumer that lists after
// writing has to expect the cache TTL to lapse first.
//
// ClientToken is left to the SDK, whose idempotency middleware fills it in
// when the input carries none, so a retried call does not create twice.

// CreateResource asks Cloud Control to create a resource of the given type.
// desiredState is the JSON document of the resource's properties, in the
// shape the type's CloudFormation schema defines.
func (r *CloudControlRepository) CreateResource(resourceType cfg.ResourceType, desiredState string) (*types.ProgressEvent, error) {
	return r.CreateResourceByInput(&awscloudcontrol.CreateResourceInput{
		TypeName:     aws.String(string(resourceType)),
		DesiredState: aws.String(desiredState),
	})
}

// CreateResourceByInput is CreateResource with the full SDK input, for callers
// that need RoleArn, TypeVersionId or their own ClientToken.
func (r *CloudControlRepository) CreateResourceByInput(query *awscloudcontrol.CreateResourceInput) (*types.ProgressEvent, error) {
	start := time.Now()
	resourceType := cfg.ResourceType(aws.ToString(query.TypeName))

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CreateResource", resourceType)).Inc()
	}

	resp, err := r.cloudcontrolClient().CreateResource(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CreateResource", resourceType)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.With(r.promLabels("CreateResource", resourceType)).Observe(time.Since(start).Seconds())
	}

	logProgress("CreateResource", resp.ProgressEvent)

	return resp.ProgressEvent, nil
}

// UpdateResource applies a JSON Patch (RFC 6902) document to the resource's
// current properties. Cloud Control rejects a patch touching a property the
// type's schema marks createOnly; such a change needs a delete and a create.
func (r *CloudControlRepository) UpdateResource(resourceType cfg.ResourceType, identifier string, patchDocument string) (*types.ProgressEvent, error) {
	return r.UpdateResourceByInput(&awscloudcontrol.UpdateResourceInput{
		TypeName:      aws.String(string(resourceType)),
		Identifier:    aws.String(identifier),
		PatchDocument: aws.String(patchDocument),
	})
}

// UpdateResourceByInput is UpdateResource with the full SDK input.
func (r *CloudControlRepository) UpdateResourceByInput(query *awscloudcontrol.UpdateResourceInput) (*types.ProgressEvent, error) {
	start := time.Now()
	resourceType := cfg.ResourceType(aws.ToString(query.TypeName))

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("UpdateResource", resourceType)).Inc()
	}

	resp, err := r.cloudcontrolClient().UpdateResource(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("UpdateResource", resourceType)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.With(r.promLabels("UpdateResource", resourceType)).Observe(time.Since(start).Seconds())
	}

	logProgress("UpdateResource", resp.ProgressEvent)

	return resp.ProgressEvent, nil
}

// DeleteResource asks Cloud Control to delete the resource with the given
// primary identifier, the same value Resource.Identifier carries.
func (r *CloudControlRepository) DeleteResource(resourceType cfg.ResourceType, identifier string) (*types.ProgressEvent, error) {
	return r.DeleteResourceByInput(&awscloudcontrol.DeleteResourceInput{
		TypeName:   aws.String(string(resourceType)),
		Identifier: aws.String(identifier),
	})
}

// DeleteResourceByInput is DeleteResource with the full SDK input.
func (r *CloudControlRepository) DeleteResourceByInput(query *awscloudcontrol.DeleteResourceInput) (*types.ProgressEvent, error) {
	start := time.Now()
	resourceType := cfg.ResourceType(aws.ToString(query.TypeName))

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("DeleteResource", resourceType)).Inc()
	}

	resp, err := r.cloudcontrolClient().DeleteResource(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("DeleteResource", resourceType)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.With(r.promLabels("DeleteResource", resourceType)).Observe(time.Since(start).Seconds())
	}

	logProgress("DeleteResource", resp.ProgressEvent)

	return resp.ProgressEvent, nil
}

// DescribeResourceRequest returns the current status of a resource request by
// the RequestToken a write returned. The output also carries the hook
// invocations the request triggered, which is why it is the SDK output rather
// than the bare ProgressEvent.
func (r *CloudControlRepository) DescribeResourceRequest(requestToken string) (*awscloudcontrol.GetResourceRequestStatusOutput, error) {
	query := &awscloudcontrol.GetResourceRequestStatusInput{RequestToken: aws.String(requestToken)}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetResourceRequestStatus", ccfg.ResourceTypeCloudControlRequest)).Inc()
	}

	resp, err := r.cloudcontrolClient().GetResourceRequestStatus(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetResourceRequestStatus", ccfg.ResourceTypeCloudControlRequest)).Inc()
		}

		return nil, errors.New(err)
	}

	return resp, nil
}

// RequestFailedError is returned when a resource request reached a terminal
// status other than SUCCESS. It carries the final ProgressEvent so a caller can
// branch on the handler's error code with errors.As instead of matching text:
//
//	var failed *cloudcontrol.RequestFailedError
//	if errors.As(err, &failed) && failed.Code() == types.HandlerErrorCodeNotFound { ... }
type RequestFailedError struct {
	Event *types.ProgressEvent
}

func (e *RequestFailedError) Error() string {
	if e.Event == nil {
		return "cloudcontrol: resource request failed"
	}

	return fmt.Sprintf("cloudcontrol: %s %s %s request %s %s: %s: %s",
		e.Event.Operation,
		aws.ToString(e.Event.TypeName),
		aws.ToString(e.Event.Identifier),
		aws.ToString(e.Event.RequestToken),
		e.Event.OperationStatus,
		e.Event.ErrorCode,
		aws.ToString(e.Event.StatusMessage))
}

// Code is the handler error code, empty when the event carries none (a
// cancelled request, for instance).
func (e *RequestFailedError) Code() types.HandlerErrorCode {
	if e.Event == nil {
		return ""
	}

	return e.Event.ErrorCode
}

// Status is the terminal status the request ended in: FAILED or CANCEL_COMPLETE.
func (e *RequestFailedError) Status() types.OperationStatus {
	if e.Event == nil {
		return ""
	}

	return e.Event.OperationStatus
}

func isTerminalFailure(status types.OperationStatus) bool {
	return status == types.OperationStatusFailed || status == types.OperationStatusCancelComplete
}

// WaitForResourceRequest polls a request until it reaches a terminal status or
// maxWait elapses, and returns the final ProgressEvent. Poll timing comes from
// WithPollInterval or WithWaiterOptions, else the SDK defaults.
//
// The SDK waiter alone reports a FAILED request as a bare "waiter state
// transitioned to Failure", losing the handler's StatusMessage and ErrorCode
// (AlreadyExists, NotFound, ...), which is the only thing a caller can act on.
// So on any waiter error the status is fetched once more; a request that ended
// FAILED or CANCEL_COMPLETE comes back as a *RequestFailedError wrapping that
// event, and the event is returned alongside the error in every case where the
// fetch succeeded. The event is nil only when that fetch fails too, which is
// what a timeout on an unreachable endpoint looks like.
func (r *CloudControlRepository) WaitForResourceRequest(requestToken string, maxWait time.Duration) (*types.ProgressEvent, error) {
	query := &awscloudcontrol.GetResourceRequestStatusInput{RequestToken: aws.String(requestToken)}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetResourceRequestStatus", ccfg.ResourceTypeCloudControlRequest)).Inc()
	}

	waiter := awscloudcontrol.NewResourceRequestSuccessWaiter(r.cloudcontrolClient(), r.waiterOptions...)

	resp, err := waiter.WaitForOutput(r.ctx, query, maxWait)
	if err == nil {
		return resp.ProgressEvent, nil
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequestErrors.With(r.promLabels("GetResourceRequestStatus", ccfg.ResourceTypeCloudControlRequest)).Inc()
	}

	status, statusErr := r.DescribeResourceRequest(requestToken)
	if statusErr != nil {
		return nil, errors.New(err)
	}

	event := status.ProgressEvent
	if event != nil && isTerminalFailure(event.OperationStatus) {
		return event, errors.New(&RequestFailedError{Event: event})
	}

	return event, errors.New(err)
}

// CreateResourceAndWait is CreateResource followed by WaitForResourceRequest,
// then a GetResource for the created resource's properties, so the caller gets
// the same Resource the read path produces. The returned Resource is nil when
// the request did not succeed; the error then explains why, and a
// *RequestFailedError in it means AWS rejected the change rather than the call.
func (r *CloudControlRepository) CreateResourceAndWait(resourceType cfg.ResourceType, desiredState string, maxWait time.Duration) (*Resource, error) {
	event, err := r.CreateResource(resourceType, desiredState)
	if err != nil {
		return nil, err
	}

	return r.waitAndDescribe(resourceType, event, maxWait)
}

// UpdateResourceAndWait is UpdateResource followed by WaitForResourceRequest
// and a GetResource for the resource's properties after the change.
func (r *CloudControlRepository) UpdateResourceAndWait(resourceType cfg.ResourceType, identifier string, patchDocument string, maxWait time.Duration) (*Resource, error) {
	event, err := r.UpdateResource(resourceType, identifier, patchDocument)
	if err != nil {
		return nil, err
	}

	return r.waitAndDescribe(resourceType, event, maxWait)
}

// DeleteResourceAndWait is DeleteResource followed by WaitForResourceRequest.
// There is nothing to describe afterwards, so it returns the final event.
func (r *CloudControlRepository) DeleteResourceAndWait(resourceType cfg.ResourceType, identifier string, maxWait time.Duration) (*types.ProgressEvent, error) {
	event, err := r.DeleteResource(resourceType, identifier)
	if err != nil {
		return nil, err
	}

	return r.WaitForResourceRequest(aws.ToString(event.RequestToken), maxWait)
}

func (r *CloudControlRepository) waitAndDescribe(resourceType cfg.ResourceType, accepted *types.ProgressEvent, maxWait time.Duration) (*Resource, error) {
	event, err := r.WaitForResourceRequest(aws.ToString(accepted.RequestToken), maxWait)
	if err != nil {
		return nil, err
	}

	// The identifier is only guaranteed once the request has succeeded: a
	// freshly accepted CREATE may not know it yet.
	identifier := event.Identifier
	if identifier == nil {
		identifier = accepted.Identifier
	}

	return r.describeAsResource(resourceType, identifier)
}

// describeAsResource is DescribeResource shaped like the list path's output.
func (r *CloudControlRepository) describeAsResource(resourceType cfg.ResourceType, identifier *string) (*Resource, error) {
	detail, err := r.DescribeResource(resourceType, identifier)
	if err != nil {
		return nil, err
	}

	if detail.ResourceDescription == nil {
		return nil, errors.Errorf("cloudcontrol: GetResource for %s %s returned no description",
			ccfg.ResourceTypeToString(resourceType), aws.ToString(identifier))
	}

	description := *detail.ResourceDescription
	attributes, tags, err := ParseAttributes(description)
	if err != nil {
		log.Warn().Err(err).
			Str("id", aws.ToString(identifier)).
			Str("type", ccfg.ResourceTypeToString(resourceType)).
			Msg("[CloudControlRepository.describeAsResource] failed parsing resource properties")
	}

	resource := NewResource(r.client, resourceType, description, attributes, tags)

	return &resource, nil
}

// CancelResourceRequest asks Cloud Control to cancel a pending or in-progress
// request. Only CREATE and UPDATE requests can be cancelled, and a cancelled
// create may leave the resource behind in whatever state the handler reached.
func (r *CloudControlRepository) CancelResourceRequest(requestToken string) (*types.ProgressEvent, error) {
	query := &awscloudcontrol.CancelResourceRequestInput{RequestToken: aws.String(requestToken)}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("CancelResourceRequest", ccfg.ResourceTypeCloudControlRequest)).Inc()
	}

	resp, err := r.cloudcontrolClient().CancelResourceRequest(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("CancelResourceRequest", ccfg.ResourceTypeCloudControlRequest)).Inc()
		}

		return nil, errors.New(err)
	}

	logProgress("CancelResourceRequest", resp.ProgressEvent)

	return resp.ProgressEvent, nil
}

// FindResourceRequests lists the account's resource requests in the client's
// region, newest first as AWS returns them. With no statuses it lists every
// request AWS still holds (about a week); with statuses it lists only those.
func (r *CloudControlRepository) FindResourceRequests(statuses ...types.OperationStatus) ([]types.ProgressEvent, error) {
	query := &awscloudcontrol.ListResourceRequestsInput{}
	if len(statuses) > 0 {
		query.ResourceRequestStatusFilter = &types.ResourceRequestStatusFilter{OperationStatuses: statuses}
	}

	requests := []types.ProgressEvent{}

	p := awscloudcontrol.NewListResourceRequestsPaginator(r.cloudcontrolClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListResourceRequests", ccfg.ResourceTypeCloudControlRequest)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListResourceRequests", ccfg.ResourceTypeCloudControlRequest)).Inc()
			}

			return requests, errors.New(err)
		}

		requests = append(requests, resp.ResourceRequestStatusSummaries...)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListResourceRequests", ccfg.ResourceTypeCloudControlRequest)).
			Add(float64(len(requests)))
	}

	return requests, nil
}

func logProgress(method string, event *types.ProgressEvent) {
	if event == nil {
		return
	}

	log.Debug().
		Str("typeName", aws.ToString(event.TypeName)).
		Str("identifier", aws.ToString(event.Identifier)).
		Str("requestToken", aws.ToString(event.RequestToken)).
		Str("status", string(event.OperationStatus)).
		Msgf("[CloudControlRepository.%s] request accepted", method)
}
