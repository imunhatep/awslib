package emr

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/emr"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service"
	cfgEntity "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func (r *EmrRepository) ListClustersAll() ([]Cluster, error) {
	return r.ListClustersByInput(&emr.ListClustersInput{})
}

func (r *EmrRepository) ListClustersLatest(createdAfter *time.Time) ([]Cluster, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(cfgEntity.ResourceTypeEmrCluster)).
		Msg("[EmrRepository::FindLatest] searching for clusters")

	if createdAfter == nil {
		createdAfter = service.LastDays(7)
	}

	return r.ListClustersByInput(&emr.ListClustersInput{CreatedAfter: createdAfter})
}

func (r *EmrRepository) ListClustersByInput(query *emr.ListClustersInput) ([]Cluster, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(cfgEntity.ResourceTypeEmrCluster)).
		Msg("[EmrRepository::FindAllBy] searching for clusters")

	start := time.Now()
	var clusters []Cluster

	p := emr.NewListClustersPaginator(r.client.EMR(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListClusters", cfgEntity.ResourceTypeEmrCluster)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListClusters", cfgEntity.ResourceTypeEmrCluster)).
					Inc()
			}

			return clusters, errors.New(err)
		}

		for _, summary := range resp.Clusters {
			cluster, err := r.DescribeCluster(summary.Id)
			if err != nil {
				var maxAttemptsErr *retry.MaxAttemptsError
				if errors.As(err, &maxAttemptsErr) {
					log.Warn().Msg("[EmrRepository.FindAllBy] throttling for 3 seconds")
					time.Sleep(time.Second * 3)

					// retry...
					cluster, err = r.DescribeCluster(summary.Id)
				}

				if cluster == nil {
					continue
				}
			}

			clusters = append(clusters, *cluster)
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListClusters", cfgEntity.ResourceTypeEmrCluster)).
			Add(float64(len(clusters)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListClustersByInput", cfgEntity.ResourceTypeEmrCluster)).
			Observe(time.Since(start).Seconds())
	}

	return clusters, nil
}

func (r *EmrRepository) DescribeCluster(clusterId *string) (*Cluster, error) {
	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Str("type", cfgEntity.ResourceTypeToString(cfgEntity.ResourceTypeEmrCluster)).
		Msgf("[EmrRepository::DescribeCluster] searching for cluster: %s", aws.ToString(clusterId))

	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DescribeCluster", cfgEntity.ResourceTypeEmrCluster)).
			Inc()
	}

	clusterDetails, err := r.client.EMR().DescribeCluster(
		r.ctx,
		&emr.DescribeClusterInput{ClusterId: clusterId},
	)

	if err != nil {
		log.Error().Err(err).
			Str("cluster", aws.ToString(clusterId)).
			Msg("[EmrRepository.FindCluster] failed to fetch EMR CLuster details")

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DescribeCluster", cfgEntity.ResourceTypeEmrCluster)).
				Inc()
		}

		return nil, err
	}

	cluster := NewCluster(r.client, clusterDetails.Cluster)

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("DescribeCluster", cfgEntity.ResourceTypeEmrCluster)).
			Observe(time.Since(start).Seconds())
	}

	return &cluster, nil
}
