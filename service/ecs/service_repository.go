package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	types2 "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *EcsRepository) ListServicesAll() ([]Service, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(types.ResourceTypeECSService)).
		Msg("[EcsRepository.ListServicesAll] searching for services")

	start := time.Now()

	var services []Service
	clusters, err := r.ListClustersAll()
	if err != nil {
		return services, errors.New(err)
	}

	for _, cluster := range clusters {
		clusterServices, _ := r.ListServicesByCluster(cluster)
		services = append(services, clusterServices...)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListServicesAll", types.ResourceTypeECSService)).
			Observe(time.Since(start).Seconds())
	}

	return services, nil
}

func (r *EcsRepository) ListServicesByCluster(cluster Cluster) ([]Service, error) {
	return r.ListServicesByInput(&ecs.ListServicesInput{Cluster: cluster.ClusterArn})
}

func (r *EcsRepository) ListServicesByInput(query *ecs.ListServicesInput) ([]Service, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfg.ResourceTypeToString(types.ResourceTypeECSService)).
		Msg("[EcsRepository.ListServicesByInput] searching for services")

	start := time.Now()

	var services []Service
	p := ecs.NewListServicesPaginator(r.client.ECS(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListServices", types.ResourceTypeECSService)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListServices", types.ResourceTypeECSService)).
					Inc()
			}

			return services, errors.New(err)
		}

		// list services
		serviceArnChunks := service.ChunkSliceString(resp.ServiceArns, 10)
		for _, chunk := range serviceArnChunks {
			chunkQuery := &ecs.DescribeServicesInput{
				Cluster:  query.Cluster,
				Services: chunk,
				Include:  []types2.ServiceField{types2.ServiceFieldTags},
			}

			chunkServices, _ := r.DescribeServicesByInput(chunkQuery)
			services = append(services, chunkServices...)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListServices", types.ResourceTypeECSService)).
			Add(float64(len(services)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListServicesByInput", types.ResourceTypeECSService)).
			Observe(time.Since(start).Seconds())
	}

	return services, nil
}

func (r *EcsRepository) DescribeServicesByInput(query *ecs.DescribeServicesInput) ([]Service, error) {
	start := time.Now()

	// list services
	var services []Service
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DescribeServices", types.ResourceTypeECSService)).
			Inc()
	}

	ecsServices, err := r.client.ECS().DescribeServices(r.ctx, query)
	for _, ecsService := range ecsServices.Services {
		if err != nil {
			log.Error().Err(err).
				Str("accountID", r.client.GetAccountID().String()).
				Str("region", r.client.GetRegion().String()).
				Str("type", cfg.ResourceTypeToString(types.ResourceTypeECSService)).
				Str("service", aws.ToString(ecsService.ServiceArn)).
				Msg("[EcsRepository.DescribeResources] failed to fetch EMR Service details")

			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeServices", types.ResourceTypeECSService)).
					Inc()
			}

			continue
		}

		services = append(services, NewService(r.client, ecsService))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeServices", types.ResourceTypeECSService)).
			Add(float64(len(services)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DescribeServicesByInput", types.ResourceTypeECSService)).
			Observe(time.Since(start).Seconds())
	}

	return services, nil
}
