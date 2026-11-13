package cloudfront

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// FunctionList holds a list of Function items.
type FunctionList struct {
	Items []Function
}

// Function is one stage of a CloudFront Function (the viewer-request/response
// JavaScript runtime, not Lambda@Edge).
//
// A function exists once per stage: DEVELOPMENT is what was last saved, LIVE is
// what distributions run, and the two differ until the function is published. A
// published function therefore appears twice, with the same name and ARN; the
// stage is what tells the rows apart. Functions are not taggable.
type Function struct {
	service.AbstractResource
	cftypes.FunctionSummary
}

func NewFunction(client AwsClient, summary cftypes.FunctionSummary) Function {
	e := Function{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(summary.Name),
			CreatedAt: epoch,
			Type:      ccfg.ResourceTypeCloudFrontFunction,
		},
		FunctionSummary: summary,
	}

	if summary.FunctionMetadata != nil {
		e.ARN = parseArn(summary.FunctionMetadata.FunctionARN)
		e.CreatedAt = createdOrEpoch(summary.FunctionMetadata.CreatedTime)
	}

	return e
}

func (e Function) GetName() string {
	return aws.ToString(e.Name)
}

// GetTags returns an empty map: CloudFront Functions are not taggable.
func (e Function) GetTags() map[string]string {
	return map[string]string{}
}

// GetStage returns LIVE or DEVELOPMENT.
func (e Function) GetStage() string {
	if e.FunctionMetadata == nil {
		return ""
	}

	return string(e.FunctionMetadata.Stage)
}

// GetStatus returns the function's status, e.g. UNPUBLISHED or UNASSOCIATED.
func (e Function) GetStatus() string {
	return aws.ToString(e.Status)
}

// GetRuntime returns the JavaScript runtime, e.g. cloudfront-js-2.0.
func (e Function) GetRuntime() string {
	if e.FunctionConfig == nil {
		return ""
	}

	return string(e.FunctionConfig.Runtime)
}

// GetKeyValueStoreArns returns the ARNs of the key value stores the function reads.
func (e Function) GetKeyValueStoreArns() []string {
	if e.FunctionConfig == nil || e.FunctionConfig.KeyValueStoreAssociations == nil {
		return nil
	}

	var out []string
	for _, assoc := range e.FunctionConfig.KeyValueStoreAssociations.Items {
		out = append(out, aws.ToString(assoc.KeyValueStoreARN))
	}

	return out
}
