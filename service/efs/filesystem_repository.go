package efs

import (
	"context"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	EFS() *efs.Client
}

type EfsRepository struct {
	ctx    context.Context
	client AwsClient
}

func NewEfsRepository(ctx context.Context, client AwsClient) *EfsRepository {
	repo := &EfsRepository{
		ctx:    ctx,
		client: client,
	}

	return repo
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
	return r.ListFileSystemsByInput(&efs.DescribeFileSystemsInput{})
}

func (r *EfsRepository) ListFileSystemsByInput(query *efs.DescribeFileSystemsInput) ([]FileSystem, error) {
	start := time.Now()
	var filesystems []FileSystem

	p := efs.NewDescribeFileSystemsPaginator(r.client.EFS(), query)
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
