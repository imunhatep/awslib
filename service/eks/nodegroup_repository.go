package eks

import (
	stderrors "errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awseks "github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// No EKS list call returns node group details: ListNodegroups answers with names, so a listing
// costs one DescribeNodegroup per node group. nodegroupFetchConcurrency and
// nodegroupFetchMaxAttempts keep those reads narrow and give them a retry budget, so a large
// account is throttled into waiting rather than into a failed listing.
const (
	nodegroupFetchConcurrency = 4
	nodegroupFetchMaxAttempts = 10
)

type nodegroupRef struct {
	cluster string
	name    string
}

// ListNodegroupsAll returns the managed node groups of every cluster in the region.
//
// Any failed call fails the listing: a node group left out is indistinguishable from one that
// does not exist. The one exception is a cluster or node group deleted between the list and the
// read (ResourceNotFoundException), which is gone, not unread.
func (r *EksRepository) ListNodegroupsAll() ([]Nodegroup, error) {
	start := time.Now()

	clusters, err := r.listClusterNames()
	if err != nil {
		return nil, err
	}

	perCluster := make([][]nodegroupRef, len(clusters))
	err = forEachBounded(len(clusters), func(i int) error {
		refs, err := r.listNodegroupRefs(clusters[i])
		perCluster[i] = refs

		return err
	})
	if err != nil {
		return nil, err
	}

	var refs []nodegroupRef
	for _, clusterRefs := range perCluster {
		refs = append(refs, clusterRefs...)
	}

	nodegroups, err := r.describeNodegroups(refs)
	if err != nil {
		return nil, err
	}

	r.observeNodegroups("ListNodegroupsAll", len(nodegroups), start)

	return nodegroups, nil
}

// ListNodegroupsByCluster returns the managed node groups of one cluster, with the same
// failure rule as ListNodegroupsAll. A cluster that does not exist yields an empty list.
func (r *EksRepository) ListNodegroupsByCluster(clusterName string) ([]Nodegroup, error) {
	if clusterName == "" {
		return nil, errors.New("cluster name cannot be empty")
	}

	start := time.Now()

	refs, err := r.listNodegroupRefs(clusterName)
	if err != nil {
		return nil, err
	}

	nodegroups, err := r.describeNodegroups(refs)
	if err != nil {
		return nil, err
	}

	r.observeNodegroups("ListNodegroupsByCluster", len(nodegroups), start)

	return nodegroups, nil
}

// listClusterNames pages through ListClusters.
func (r *EksRepository) listClusterNames() ([]string, error) {
	var names []string

	p := awseks.NewListClustersPaginator(r.eksClient(), &awseks.ListClustersInput{})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListClusters", ccfg.ResourceTypeEKSNodegroup)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListClusters", ccfg.ResourceTypeEKSNodegroup)).
					Inc()
			}

			return nil, errors.New(err)
		}

		names = append(names, resp.Clusters...)
	}

	return names, nil
}

// listNodegroupRefs pages through ListNodegroups for one cluster. A cluster deleted since it
// was listed has no node groups left to report.
func (r *EksRepository) listNodegroupRefs(clusterName string) ([]nodegroupRef, error) {
	var refs []nodegroupRef

	p := awseks.NewListNodegroupsPaginator(r.eksClient(), &awseks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("ListNodegroups", ccfg.ResourceTypeEKSNodegroup)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if isNotFound(err) {
				log.Debug().Str("cluster", clusterName).
					Msg("[EksRepository.listNodegroupRefs] cluster no longer exists")

				return nil, nil
			}

			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("ListNodegroups", ccfg.ResourceTypeEKSNodegroup)).
					Inc()
			}

			return nil, errors.Errorf("listing node groups of cluster %s: %w", clusterName, err)
		}

		for _, name := range resp.Nodegroups {
			refs = append(refs, nodegroupRef{cluster: clusterName, name: name})
		}
	}

	return refs, nil
}

// describeNodegroups reads every node group, bounded by nodegroupFetchConcurrency, and keeps
// the input order.
func (r *EksRepository) describeNodegroups(refs []nodegroupRef) ([]Nodegroup, error) {
	described := make([]*types.Nodegroup, len(refs))

	err := forEachBounded(len(refs), func(i int) error {
		nodegroup, err := r.describeNodegroup(refs[i])
		described[i] = nodegroup

		return err
	})
	if err != nil {
		return nil, err
	}

	nodegroups := make([]Nodegroup, 0, len(refs))
	for _, nodegroup := range described {
		if nodegroup != nil {
			nodegroups = append(nodegroups, NewNodegroup(r.client, *nodegroup))
		}
	}

	return nodegroups, nil
}

// describeNodegroup reads one node group; nil without an error when it no longer exists.
func (r *EksRepository) describeNodegroup(ref nodegroupRef) (*types.Nodegroup, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DescribeNodegroup", ccfg.ResourceTypeEKSNodegroup)).
			Inc()
	}

	resp, err := r.eksClient().DescribeNodegroup(r.ctx, &awseks.DescribeNodegroupInput{
		ClusterName:   aws.String(ref.cluster),
		NodegroupName: aws.String(ref.name),
	}, func(o *awseks.Options) { o.RetryMaxAttempts = nodegroupFetchMaxAttempts })
	if err != nil {
		if isNotFound(err) {
			log.Debug().Str("cluster", ref.cluster).Str("nodegroup", ref.name).
				Msg("[EksRepository.describeNodegroup] node group no longer exists")

			return nil, nil
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DescribeNodegroup", ccfg.ResourceTypeEKSNodegroup)).
				Inc()
		}

		return nil, errors.Errorf("describing node group %s/%s: %w", ref.cluster, ref.name, err)
	}

	return resp.Nodegroup, nil
}

func (r *EksRepository) observeNodegroups(method string, count int, start time.Time) {
	if !metrics.AwsMetricsEnabled {
		return
	}

	metrics.AwsApiResourcesFetched.
		With(r.promLabels("DescribeNodegroup", ccfg.ResourceTypeEKSNodegroup)).
		Add(float64(count))

	metrics.AwsRepoCallDuration.
		With(r.promLabels(method, ccfg.ResourceTypeEKSNodegroup)).
		Observe(time.Since(start).Seconds())
}

func isNotFound(err error) bool {
	var notFound *types.ResourceNotFoundException
	return stderrors.As(err, &notFound)
}

// forEachBounded runs fn for every index with at most nodegroupFetchConcurrency in flight, and
// returns the first error by index.
func forEachBounded(n int, fn func(i int) error) error {
	errs := make([]error, n)
	sem := make(chan struct{}, nodegroupFetchConcurrency)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			errs[idx] = fn(idx)
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
