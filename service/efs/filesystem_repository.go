package efs

import (
	"context"
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsefs "github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/efs"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type EfsRepository struct {
	ctx    context.Context
	client *v3.Client
}

func NewEfsRepository(ctx context.Context, client *v3.Client) *EfsRepository {
	repo := &EfsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *EfsRepository) efsClient() *awsefs.Client {
	return efs.GetClient(r.client)
}

func (r *EfsRepository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *EfsRepository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *EfsRepository) ListFileSystemsAll() ([]FileSystem, error) {
	return r.ListFileSystemsByInput(&awsefs.DescribeFileSystemsInput{})
}

func (r *EfsRepository) ListFileSystemsByInput(query *awsefs.DescribeFileSystemsInput) ([]FileSystem, error) {
	start := time.Now()
	var filesystems []FileSystem

	p := awsefs.NewDescribeFileSystemsPaginator(r.efsClient(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("DescribeFileSystems", cfg.ResourceTypeEFSFileSystem)).
				Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.
					With(r.promLabels("DescribeFileSystems", cfg.ResourceTypeEFSFileSystem)).
					Inc()
			}

			return filesystems, errors.New(err)
		}

		for _, v := range resp.FileSystems {
			filesystems = append(filesystems, NewFileSystem(r.client, v))
		}
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeFileSystems", cfg.ResourceTypeEFSFileSystem)).
			Add(float64(len(filesystems)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListFileSystemsByInput", cfg.ResourceTypeEFSFileSystem)).
			Observe(time.Since(start).Seconds())
	}

	return filesystems, nil
}
