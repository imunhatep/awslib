package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (r *Ec2Repository) ListRegionsAll() ([]types.Region, error) {
	return r.ListRegionByInput(&ec2.DescribeRegionsInput{})
}

func (r *Ec2Repository) ListRegionsOptIn() ([]types.Region, error) {
	query := &ec2.DescribeRegionsInput{
		Filters: []types.Filter{
			{Name: aws.String("opt-in-status"), Values: []string{"opted-in"}},
		},
	}

	return r.ListRegionByInput(query)
}

func (r *Ec2Repository) ListRegionByInput(query *ec2.DescribeRegionsInput) ([]types.Region, error) {
	res, err := r.client.EC2().DescribeRegions(r.ctx, query)
	if err != nil {
		return nil, err
	}

	return res.Regions, nil
}
