package proxy

import (
	"context"

	"github.com/imunhatep/awslib/cache"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/ec2"
	"github.com/imunhatep/awslib/service/ecr"
	"github.com/imunhatep/awslib/service/eks"
	"github.com/imunhatep/awslib/service/elb"
	"github.com/imunhatep/awslib/service/kms"
	"github.com/imunhatep/awslib/service/rds"
	"github.com/imunhatep/gocollection/slice"
)

// FindEc2NatGateways returns a list of NAT gateways with their addresses
func FindEc2NatGateways(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListNatGatewaysAll()
		return slice.Map(items, cast[ec2.NatGateway]), err
	}
	items, err := repo.ListNatGatewaysAll()
	return slice.Map(items, cast[ec2.NatGateway]), err
}

// FindEc2InternetGateways returns a list of internet gateways and their VPC attachments
func FindEc2InternetGateways(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListInternetGatewaysAll()
		return slice.Map(items, cast[ec2.InternetGateway]), err
	}
	items, err := repo.ListInternetGatewaysAll()
	return slice.Map(items, cast[ec2.InternetGateway]), err
}

// FindEc2TransitGateways returns a list of transit gateways, including ones shared into the account
func FindEc2TransitGateways(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListTransitGatewaysAll()
		return slice.Map(items, cast[ec2.TransitGateway]), err
	}
	items, err := repo.ListTransitGatewaysAll()
	return slice.Map(items, cast[ec2.TransitGateway]), err
}

// FindEc2TransitGatewayAttachments returns a list of transit gateway attachments
func FindEc2TransitGatewayAttachments(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListTransitGatewayAttachmentsAll()
		return slice.Map(items, cast[ec2.TransitGatewayAttachment]), err
	}
	items, err := repo.ListTransitGatewayAttachmentsAll()
	return slice.Map(items, cast[ec2.TransitGatewayAttachment]), err
}

// FindEc2VpcPeeringConnections returns a list of VPC peering connections
func FindEc2VpcPeeringConnections(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListVpcPeeringConnectionsAll()
		return slice.Map(items, cast[ec2.VpcPeeringConnection]), err
	}
	items, err := repo.ListVpcPeeringConnectionsAll()
	return slice.Map(items, cast[ec2.VpcPeeringConnection]), err
}

// FindEc2Images returns the AMIs the account owns (Owners: self)
func FindEc2Images(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListImagesAll()
		return slice.Map(items, cast[ec2.Image]), err
	}
	items, err := repo.ListImagesAll()
	return slice.Map(items, cast[ec2.Image]), err
}

// FindEc2LaunchTemplates returns a list of EC2 launch templates
func FindEc2LaunchTemplates(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ec2.NewEc2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListLaunchTemplatesAll()
		return slice.Map(items, cast[ec2.LaunchTemplate]), err
	}
	items, err := repo.ListLaunchTemplatesAll()
	return slice.Map(items, cast[ec2.LaunchTemplate]), err
}

// FindElbTargetGroups returns a list of ELBv2 target groups with their targets' health
func FindElbTargetGroups(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := elb.NewLoadBalancerRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListTargetGroupsAll()
		return slice.Map(items, cast[elb.TargetGroup]), err
	}
	items, err := repo.ListTargetGroupsAll()
	return slice.Map(items, cast[elb.TargetGroup]), err
}

// FindElbListeners returns the listeners of every ELBv2 load balancer
func FindElbListeners(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := elb.NewLoadBalancerRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListListenersAll()
		return slice.Map(items, cast[elb.Listener]), err
	}
	items, err := repo.ListListenersAll()
	return slice.Map(items, cast[elb.Listener]), err
}

// FindEksNodegroups returns the managed node groups of every EKS cluster
func FindEksNodegroups(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := eks.NewEksRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListNodegroupsAll()
		return slice.Map(items, cast[eks.Nodegroup]), err
	}
	items, err := repo.ListNodegroupsAll()
	return slice.Map(items, cast[eks.Nodegroup]), err
}

// FindRdsDBClusters returns a list of RDS clusters (Aurora, Neptune, DocumentDB)
func FindRdsDBClusters(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := rds.NewRdsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListDBClustersAll()
		return slice.Map(items, cast[rds.DBCluster]), err
	}
	items, err := repo.ListDBClustersAll()
	return slice.Map(items, cast[rds.DBCluster]), err
}

// FindRdsDBClusterSnapshots returns the account's manual and automated RDS cluster snapshots
func FindRdsDBClusterSnapshots(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := rds.NewRdsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListDBClusterSnapshotsAll()
		return slice.Map(items, cast[rds.DBClusterSnapshot]), err
	}
	items, err := repo.ListDBClusterSnapshotsAll()
	return slice.Map(items, cast[rds.DBClusterSnapshot]), err
}

// FindKmsKeys returns a list of KMS keys with their aliases
func FindKmsKeys(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := kms.NewKmsRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListKeysAll()
		return slice.Map(items, cast[kms.Key]), err
	}
	items, err := repo.ListKeysAll()
	return slice.Map(items, cast[kms.Key]), err
}

// FindEcrRepositories returns a list of ECR repositories
func FindEcrRepositories(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := ecr.NewEcrRepository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListRepositoriesAll()
		return slice.Map(items, cast[ecr.Repo]), err
	}
	items, err := repo.ListRepositoriesAll()
	return slice.Map(items, cast[ecr.Repo]), err
}
