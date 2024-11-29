package cloudcontrol

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
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
	CloudControl() *cloudcontrol.Client
}

type CloudControlRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewCloudControlRepository(ctx context.Context, client AwsClient) *CloudControlRepository {
	repo := &CloudControlRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *CloudControlRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *CloudControlRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *CloudControlRepository) FindResources(query *cloudcontrol.ListResourcesInput) ([]types.ResourceDescription, error) {
	resourceType := cfg.ResourceType(aws.ToString(query.TypeName))
	resources := []types.ResourceDescription{}

	p := cloudcontrol.NewListResourcesPaginator(r.client.CloudControl(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListResources", cfg.ResourceType(aws.ToString(query.TypeName)))).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListResources", resourceType)).
					Inc()
			}

			return resources, errors.New(err)
		}

		resources = append(resources, resp.ResourceDescriptions...)

		if query.MaxResults != nil && int32(len(resources)) >= *query.MaxResults {
			break
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListResources", resourceType)).
			Add(float64(len(resources)))
	}

	if query.MaxResults != nil {
		return resources[:*query.MaxResults], nil
	}

	return resources, nil
}

func (r *CloudControlRepository) DescribeResource(resourceType cfg.ResourceType, identifier *string) (*cloudcontrol.GetResourceOutput, error) {
	start := time.Now()

	query := &cloudcontrol.GetResourceInput{
		Identifier: identifier,
		TypeName:   aws.String(string(resourceType)),
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetResource", resourceType)).
			Inc()
	}

	resp, err := r.client.CloudControl().GetResource(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("GetResource", resourceType)).
				Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels("GetResource", resourceType)).Add(1)
		metrics.AwsRepoCallDuration.With(r.promLabels("DescribeResource", resourceType)).Observe(time.Since(start).Seconds())
	}

	return resp, nil
}

func ParseAttributes(resource types.ResourceDescription) (data map[string]interface{}, tags map[string]string, err error) {
	// parse attributes
	err = json.Unmarshal([]byte(aws.ToString(resource.Properties)), &data)

	// parse tags from attributes
	if err == nil {
		tags, err = ParseTags(data)
	}

	return data, tags, err
}

func ParseTags(attrs map[string]interface{}) (map[string]string, error) {
	tags := map[string]string{}

	rawTags, ok := attrs["Tags"]
	if !ok {
		return tags, nil
	}

	tagList, ok := rawTags.([]interface{})
	if !ok {
		log.Warn().Msg("[CloudControl.ParseTags] resource tags are not a list of maps")
		return tags, errors.New("resource tags are not a list")
	}

	for _, item := range tagList {
		tagMap, ok := item.(map[string]interface{})
		if !ok {
			log.Warn().Msg("[CloudControl.ParseTags] tag entry is not a map")
			continue
		}

		key, keyOK := tagMap["Key"]
		value, valOK := tagMap["Value"]

		if !keyOK || !valOK {
			continue
		}

		tags[key.(string)] = value.(string)
	}

	return tags, nil
}
