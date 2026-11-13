package cloudfront

import (
	"time"

	awscf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// functionStages are listed separately by ListFunctionsAll. ListFunctions
// filters on one stage at a time, and a request without one is not documented
// to return both, so asking for each explicitly is what guarantees that a
// function which was never published (DEVELOPMENT only) and the code that is
// actually serving traffic (LIVE) both show up.
var functionStages = []cftypes.FunctionStage{cftypes.FunctionStageLive, cftypes.FunctionStageDevelopment}

// ListFunctionsAll lists every CloudFront Function in both stages, LIVE first.
func (r *CloudFrontRepository) ListFunctionsAll() ([]Function, error) {
	var out []Function

	for _, stage := range functionStages {
		functions, err := r.ListFunctionsByInput(&awscf.ListFunctionsInput{Stage: stage})
		if err != nil {
			return nil, err
		}

		out = append(out, functions...)
	}

	return out, nil
}

// ListFunctionsByInput lists CloudFront Functions, following NextMarker to the end.
func (r *CloudFrontRepository) ListFunctionsByInput(query *awscf.ListFunctionsInput) ([]Function, error) {
	start := time.Now()

	input := awscf.ListFunctionsInput{}
	if query != nil {
		input = *query
	}

	summaries, err := listPages(r, "ListFunctions", ccfg.ResourceTypeCloudFrontFunction, input.Marker,
		func(marker *string) ([]cftypes.FunctionSummary, *string, error) {
			page := input
			page.Marker = marker

			output, err := r.cloudFrontClient().ListFunctions(r.ctx, &page)
			if err != nil {
				return nil, nil, err
			}

			if output.FunctionList == nil {
				return nil, nil, nil
			}

			return output.FunctionList.Items, output.FunctionList.NextMarker, nil
		})
	if err != nil {
		return nil, err
	}

	functions := make([]Function, 0, len(summaries))
	for _, summary := range summaries {
		functions = append(functions, NewFunction(r.client, summary))
	}

	r.observeList("ListFunctions", "ListFunctionsByInput", ccfg.ResourceTypeCloudFrontFunction, len(functions), start)

	return functions, nil
}
