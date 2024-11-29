package lambda

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"time"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	Lambda() *lambda.Client
}

type LambdaRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewLambdaRepository(ctx context.Context, client AwsClient) *LambdaRepository {
	repo := &LambdaRepository{ctx, client}

	return repo
}

func (r *LambdaRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *LambdaRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *LambdaRepository) ListFunctionsAll() ([]Function, error) {
	return r.ListFunctionsByInput(&lambda.ListFunctionsInput{})
}

func (r *LambdaRepository) ListFunctionsByInput(query *lambda.ListFunctionsInput) ([]Function, error) {
	start := time.Now()
	var functions []Function

	p := lambda.NewListFunctionsPaginator(r.client.Lambda(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListFunctions", cfg.ResourceTypeFunction)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListFunctions", cfg.ResourceTypeFunction)).Inc()
			}

			return functions, errors.New(err)
		}

		for _, v := range resp.Functions {
			tags, _ := r.ListFunctionTags(v)
			secret := NewFunction(r.client, v, tags)
			functions = append(functions, secret)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListFunctions", cfg.ResourceTypeFunction)).
			Add(float64(len(functions)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListFunctionsByInput", cfg.ResourceTypeFunction)).
			Observe(time.Since(start).Seconds())
	}

	return functions, nil
}

func (r *LambdaRepository) ListFunctionTags(fn types.FunctionConfiguration) (map[string]string, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListFunctionTags", cfg.ResourceTypeFunction)).Inc()
	}

	tagOutput, err := r.client.Lambda().ListTags(r.ctx, &lambda.ListTagsInput{Resource: fn.FunctionArn})

	if err != nil {
		log.Debug().Str("function", aws.ToString(fn.FunctionName)).Err(err).Msg("failed to fetch lambda tags")
		return map[string]string{}, errors.New(err)
	}

	return tagOutput.Tags, nil
}
