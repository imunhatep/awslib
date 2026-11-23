package ec2

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var networkingCreated = time.Date(2024, 3, 1, 12, 30, 0, 0, time.UTC)

func nameTag(name string) []types.Tag {
	return []types.Tag{{Key: aws.String("Name"), Value: aws.String(name)}}
}

func TestEc2NetworkingResourceInterface(t *testing.T) {
	client := mockClient{}

	tests := []struct {
		name      string
		resource  service.ResourceInterface
		id        string
		arn       string
		rType     cfg.ResourceType
		rName     string
		createdAt time.Time
	}{
		{
			name: "nat gateway",
			resource: NewNatGateway(client, types.NatGateway{
				NatGatewayId: aws.String("nat-0a1b2c"),
				CreateTime:   aws.Time(networkingCreated),
				Tags:         nameTag("egress-a"),
			}),
			id:        "nat-0a1b2c",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:natgateway/nat-0a1b2c",
			rType:     cfg.ResourceTypeNatGateway,
			rName:     "egress-a",
			createdAt: networkingCreated,
		},
		{
			name:      "nat gateway without a name tag",
			resource:  NewNatGateway(client, types.NatGateway{NatGatewayId: aws.String("nat-2")}),
			id:        "nat-2",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:natgateway/nat-2",
			rType:     cfg.ResourceTypeNatGateway,
			rName:     "-",
			createdAt: time.Time{},
		},
		{
			name: "internet gateway has no creation time",
			resource: NewInternetGateway(client, types.InternetGateway{
				InternetGatewayId: aws.String("igw-0a1b2c"),
				Tags:              nameTag("main-igw"),
			}),
			id:        "igw-0a1b2c",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:internet-gateway/igw-0a1b2c",
			rType:     cfg.ResourceTypeInternetGateway,
			rName:     "main-igw",
			createdAt: time.Unix(0, 0),
		},
		{
			name: "transit gateway uses the ARN returned by the API",
			resource: NewTransitGateway(client, types.TransitGateway{
				TransitGatewayId:  aws.String("tgw-0a1b2c"),
				TransitGatewayArn: aws.String("arn:aws:ec2:eu-west-1:210987654321:transit-gateway/tgw-0a1b2c"),
				CreationTime:      aws.Time(networkingCreated),
				Tags:              nameTag("core-tgw"),
			}),
			id:        "tgw-0a1b2c",
			arn:       "arn:aws:ec2:eu-west-1:210987654321:transit-gateway/tgw-0a1b2c",
			rType:     cfg.ResourceTypeTransitGateway,
			rName:     "core-tgw",
			createdAt: networkingCreated,
		},
		{
			name:      "transit gateway without an API ARN falls back to a synthesized one",
			resource:  NewTransitGateway(client, types.TransitGateway{TransitGatewayId: aws.String("tgw-2")}),
			id:        "tgw-2",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:transit-gateway/tgw-2",
			rType:     cfg.ResourceTypeTransitGateway,
			rName:     "-",
			createdAt: time.Time{},
		},
		{
			name: "transit gateway attachment",
			resource: NewTransitGatewayAttachment(client, types.TransitGatewayAttachment{
				TransitGatewayAttachmentId: aws.String("tgw-attach-0a1b2c"),
				CreationTime:               aws.Time(networkingCreated),
				Tags:                       nameTag("vpc-a-to-core"),
			}),
			id:        "tgw-attach-0a1b2c",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:transit-gateway-attachment/tgw-attach-0a1b2c",
			rType:     cfg.ResourceTypeTransitGatewayAttachment,
			rName:     "vpc-a-to-core",
			createdAt: networkingCreated,
		},
		{
			name: "vpc peering connection has no creation time",
			resource: NewVpcPeeringConnection(client, types.VpcPeeringConnection{
				VpcPeeringConnectionId: aws.String("pcx-0a1b2c"),
				Tags:                   nameTag("a-to-b"),
			}),
			id:        "pcx-0a1b2c",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:vpc-peering-connection/pcx-0a1b2c",
			rType:     cfg.ResourceTypeVPCPeeringConnection,
			rName:     "a-to-b",
			createdAt: time.Unix(0, 0),
		},
		{
			name: "image falls back to the AMI name and parses its creation date",
			resource: NewImage(client, types.Image{
				ImageId:      aws.String("ami-0a1b2c"),
				Name:         aws.String("golden-2024-03-01"),
				CreationDate: aws.String("2024-03-01T12:30:00.000Z"),
			}),
			id:        "ami-0a1b2c",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:image/ami-0a1b2c",
			rType:     ccfg.ResourceTypeEc2Image,
			rName:     "golden-2024-03-01",
			createdAt: networkingCreated,
		},
		{
			name: "image prefers the name tag and keeps the epoch on a bad date",
			resource: NewImage(client, types.Image{
				ImageId:      aws.String("ami-2"),
				Name:         aws.String("golden"),
				CreationDate: aws.String("yesterday"),
				Tags:         nameTag("tagged-golden"),
			}),
			id:        "ami-2",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:image/ami-2",
			rType:     ccfg.ResourceTypeEc2Image,
			rName:     "tagged-golden",
			createdAt: time.Unix(0, 0),
		},
		{
			name: "launch template falls back to the template name",
			resource: NewLaunchTemplate(client, types.LaunchTemplate{
				LaunchTemplateId:   aws.String("lt-0a1b2c"),
				LaunchTemplateName: aws.String("web-nodes"),
				CreateTime:         aws.Time(networkingCreated),
			}),
			id:        "lt-0a1b2c",
			arn:       "arn:aws:ec2:eu-west-1:123456789012:launch-template/lt-0a1b2c",
			rType:     cfg.ResourceTypeLaunchTemplate,
			rName:     "web-nodes",
			createdAt: networkingCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.id, tt.resource.GetId())
			assert.Equal(t, tt.arn, tt.resource.GetArn())
			assert.Equal(t, tt.rType, tt.resource.GetType())
			assert.Equal(t, tt.rName, tt.resource.GetName())
			assert.True(t, tt.createdAt.Equal(tt.resource.GetCreatedAt()), "created at %s, want %s", tt.resource.GetCreatedAt(), tt.createdAt)
			assert.Equal(t, ptypes.AwsAccountID("123456789012"), tt.resource.GetAccountID())
			assert.Equal(t, ptypes.AwsRegion("eu-west-1"), tt.resource.GetRegion())
		})
	}
}

func TestEc2NetworkingGetTagsReturnsCopy(t *testing.T) {
	client := mockClient{}
	resources := []service.ResourceInterface{
		NewNatGateway(client, types.NatGateway{NatGatewayId: aws.String("nat-1"), Tags: nameTag("a")}),
		NewInternetGateway(client, types.InternetGateway{InternetGatewayId: aws.String("igw-1"), Tags: nameTag("a")}),
		NewTransitGateway(client, types.TransitGateway{TransitGatewayId: aws.String("tgw-1"), Tags: nameTag("a")}),
		NewTransitGatewayAttachment(client, types.TransitGatewayAttachment{TransitGatewayAttachmentId: aws.String("tgw-attach-1"), Tags: nameTag("a")}),
		NewVpcPeeringConnection(client, types.VpcPeeringConnection{VpcPeeringConnectionId: aws.String("pcx-1"), Tags: nameTag("a")}),
		NewImage(client, types.Image{ImageId: aws.String("ami-1"), Tags: nameTag("a")}),
		NewLaunchTemplate(client, types.LaunchTemplate{LaunchTemplateId: aws.String("lt-1"), Tags: nameTag("a")}),
	}

	for _, r := range resources {
		r.GetTags()["Name"] = "mutated"
		assert.Equal(t, "a", r.GetTags()["Name"], string(r.GetType()))
	}
}

func TestNatGatewayAddresses(t *testing.T) {
	nat := NewNatGateway(mockClient{}, types.NatGateway{
		NatGatewayId:     aws.String("nat-1"),
		VpcId:            aws.String("vpc-1"),
		SubnetId:         aws.String("subnet-1"),
		State:            types.NatGatewayStateAvailable,
		ConnectivityType: types.ConnectivityTypePublic,
		NatGatewayAddresses: []types.NatGatewayAddress{
			{
				AllocationId:       aws.String("eipalloc-1"),
				NetworkInterfaceId: aws.String("eni-1"),
				PrivateIp:          aws.String("10.0.0.10"),
				PublicIp:           aws.String("52.1.2.3"),
				IsPrimary:          aws.Bool(true),
			},
			{
				AllocationId:       aws.String("eipalloc-2"),
				NetworkInterfaceId: aws.String("eni-1"),
				PrivateIp:          aws.String("10.0.0.11"),
				PublicIp:           aws.String("52.1.2.4"),
			},
		},
	})

	assert.Equal(t, "vpc-1", nat.GetVpcId())
	assert.Equal(t, "subnet-1", nat.GetSubnetId())
	assert.Equal(t, "available", nat.GetState())
	assert.Equal(t, "public", nat.GetConnectivityType())
	assert.True(t, nat.IsPublic())
	assert.Equal(t, []string{"52.1.2.3", "52.1.2.4"}, nat.GetPublicIps())
	assert.Equal(t, []string{"10.0.0.10", "10.0.0.11"}, nat.GetPrivateIps())
	assert.Equal(t, []string{"eipalloc-1", "eipalloc-2"}, nat.GetAllocationIds())
	assert.Equal(t, []string{"eni-1"}, nat.GetNetworkInterfaceIds(), "secondary addresses share the interface")

	private := NewNatGateway(mockClient{}, types.NatGateway{
		NatGatewayId:        aws.String("nat-2"),
		ConnectivityType:    types.ConnectivityTypePrivate,
		NatGatewayAddresses: []types.NatGatewayAddress{{PrivateIp: aws.String("10.0.0.12"), NetworkInterfaceId: aws.String("eni-2")}},
	})
	assert.False(t, private.IsPublic())
	assert.Equal(t, []string{}, private.GetPublicIps())
	assert.Equal(t, []string{}, private.GetAllocationIds())
	assert.Equal(t, []string{"10.0.0.12"}, private.GetPrivateIps())
}

func TestInternetGatewayAttachments(t *testing.T) {
	attached := NewInternetGateway(mockClient{}, types.InternetGateway{
		InternetGatewayId: aws.String("igw-1"),
		OwnerId:           aws.String("123456789012"),
		Attachments:       []types.InternetGatewayAttachment{{VpcId: aws.String("vpc-1"), State: types.AttachmentStatusAttached}},
	})
	assert.True(t, attached.IsAttached())
	assert.Equal(t, []string{"vpc-1"}, attached.GetAttachedVpcIds())
	assert.Equal(t, "123456789012", attached.GetOwnerId())

	detached := NewInternetGateway(mockClient{}, types.InternetGateway{InternetGatewayId: aws.String("igw-2")})
	assert.False(t, detached.IsAttached())
	assert.Equal(t, []string{}, detached.GetAttachedVpcIds())
}

func TestTransitGatewayOptions(t *testing.T) {
	tgw := NewTransitGateway(mockClient{}, types.TransitGateway{
		TransitGatewayId: aws.String("tgw-1"),
		OwnerId:          aws.String("210987654321"),
		State:            types.TransitGatewayStateAvailable,
		Description:      aws.String("core"),
		Options: &types.TransitGatewayOptions{
			AmazonSideAsn:                  aws.Int64(64512),
			AssociationDefaultRouteTableId: aws.String("tgw-rtb-1"),
			PropagationDefaultRouteTableId: aws.String("tgw-rtb-2"),
			AutoAcceptSharedAttachments:    types.AutoAcceptSharedAttachmentsValueEnable,
			TransitGatewayCidrBlocks:       []string{"10.255.0.0/24"},
		},
	})

	assert.Equal(t, "210987654321", tgw.GetOwnerId())
	assert.Equal(t, "available", tgw.GetState())
	assert.Equal(t, "core", tgw.GetDescription())
	assert.Equal(t, int64(64512), tgw.GetAmazonSideAsn())
	assert.Equal(t, "tgw-rtb-1", tgw.GetAssociationDefaultRouteTableId())
	assert.Equal(t, "tgw-rtb-2", tgw.GetPropagationDefaultRouteTableId())
	assert.True(t, tgw.IsAutoAcceptSharedAttachments())
	assert.Equal(t, []string{"10.255.0.0/24"}, tgw.GetCidrBlocks())

	bare := NewTransitGateway(mockClient{}, types.TransitGateway{TransitGatewayId: aws.String("tgw-2")})
	assert.Equal(t, int64(0), bare.GetAmazonSideAsn())
	assert.Equal(t, "", bare.GetAssociationDefaultRouteTableId())
	assert.False(t, bare.IsAutoAcceptSharedAttachments())
	assert.Equal(t, []string{}, bare.GetCidrBlocks())
}

func TestTransitGatewayAttachmentFields(t *testing.T) {
	attachment := NewTransitGatewayAttachment(mockClient{}, types.TransitGatewayAttachment{
		TransitGatewayAttachmentId: aws.String("tgw-attach-1"),
		TransitGatewayId:           aws.String("tgw-1"),
		TransitGatewayOwnerId:      aws.String("210987654321"),
		ResourceType:               types.TransitGatewayAttachmentResourceTypeVpc,
		ResourceId:                 aws.String("vpc-1"),
		ResourceOwnerId:            aws.String("123456789012"),
		State:                      types.TransitGatewayAttachmentStateAvailable,
		Association: &types.TransitGatewayAttachmentAssociation{
			TransitGatewayRouteTableId: aws.String("tgw-rtb-1"),
			State:                      types.TransitGatewayAssociationStateAssociated,
		},
	})

	assert.Equal(t, "tgw-1", attachment.GetTransitGatewayId())
	assert.Equal(t, "210987654321", attachment.GetTransitGatewayOwnerId())
	assert.Equal(t, "vpc", attachment.GetResourceType())
	assert.Equal(t, "vpc-1", attachment.GetResourceId())
	assert.Equal(t, "123456789012", attachment.GetResourceOwnerId())
	assert.Equal(t, "available", attachment.GetState())
	assert.Equal(t, "tgw-rtb-1", attachment.GetRouteTableId())
	assert.Equal(t, "associated", attachment.GetAssociationState())
	assert.True(t, attachment.IsCrossAccount())

	unassociated := NewTransitGatewayAttachment(mockClient{}, types.TransitGatewayAttachment{TransitGatewayAttachmentId: aws.String("tgw-attach-2")})
	assert.Equal(t, "", unassociated.GetRouteTableId())
	assert.Equal(t, "", unassociated.GetAssociationState())
	assert.False(t, unassociated.IsCrossAccount())
}

func TestVpcPeeringConnectionSides(t *testing.T) {
	peering := NewVpcPeeringConnection(mockClient{}, types.VpcPeeringConnection{
		VpcPeeringConnectionId: aws.String("pcx-1"),
		Status:                 &types.VpcPeeringConnectionStateReason{Code: types.VpcPeeringConnectionStateReasonCodeActive, Message: aws.String("Active")},
		RequesterVpcInfo: &types.VpcPeeringConnectionVpcInfo{
			VpcId:     aws.String("vpc-a"),
			OwnerId:   aws.String("123456789012"),
			Region:    aws.String("eu-west-1"),
			CidrBlock: aws.String("10.0.0.0/16"),
		},
		AccepterVpcInfo: &types.VpcPeeringConnectionVpcInfo{
			VpcId:     aws.String("vpc-b"),
			OwnerId:   aws.String("210987654321"),
			Region:    aws.String("us-east-1"),
			CidrBlock: aws.String("10.1.0.0/16"),
		},
	})

	assert.Equal(t, "active", peering.GetStatusCode())
	assert.Equal(t, "Active", peering.GetStatusMessage())
	assert.Equal(t, "vpc-a", peering.GetRequesterVpcId())
	assert.Equal(t, "123456789012", peering.GetRequesterOwnerId())
	assert.Equal(t, "eu-west-1", peering.GetRequesterRegion())
	assert.Equal(t, "10.0.0.0/16", peering.GetRequesterCidrBlock())
	assert.Equal(t, "vpc-b", peering.GetAccepterVpcId())
	assert.Equal(t, "210987654321", peering.GetAccepterOwnerId())
	assert.Equal(t, "us-east-1", peering.GetAccepterRegion())
	assert.Equal(t, "10.1.0.0/16", peering.GetAccepterCidrBlock())
	assert.True(t, peering.IsCrossAccount())
	assert.True(t, peering.IsCrossRegion())

	bare := NewVpcPeeringConnection(mockClient{}, types.VpcPeeringConnection{VpcPeeringConnectionId: aws.String("pcx-2")})
	assert.Equal(t, "", bare.GetStatusCode())
	assert.Equal(t, "", bare.GetRequesterVpcId())
	assert.Equal(t, "", bare.GetAccepterCidrBlock())
	assert.False(t, bare.IsCrossAccount())
	assert.False(t, bare.IsCrossRegion())
}

func TestImageSnapshotsAndTimes(t *testing.T) {
	image := NewImage(mockClient{}, types.Image{
		ImageId:          aws.String("ami-1"),
		Name:             aws.String("golden"),
		Description:      aws.String("base image"),
		State:            types.ImageStateAvailable,
		OwnerId:          aws.String("123456789012"),
		Public:           aws.Bool(true),
		Architecture:     types.ArchitectureValuesArm64,
		PlatformDetails:  aws.String("Linux/UNIX"),
		RootDeviceType:   types.DeviceTypeEbs,
		LastLaunchedTime: aws.String("2024-05-01T00:00:00Z"),
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{DeviceName: aws.String("/dev/xvda"), Ebs: &types.EbsBlockDevice{SnapshotId: aws.String("snap-1"), VolumeSize: aws.Int32(8)}},
			{DeviceName: aws.String("/dev/xvdb"), Ebs: &types.EbsBlockDevice{SnapshotId: aws.String("snap-2"), VolumeSize: aws.Int32(100)}},
			{DeviceName: aws.String("/dev/sdc"), VirtualName: aws.String("ephemeral0")},
		},
	})

	assert.Equal(t, []string{"snap-1", "snap-2"}, image.GetSnapshotIds())
	assert.Equal(t, int32(108), image.GetSnapshotSizeGiB())
	assert.Equal(t, "golden", image.GetImageName())
	assert.Equal(t, "base image", image.GetDescription())
	assert.Equal(t, "available", image.GetState())
	assert.Equal(t, "123456789012", image.GetOwnerId())
	assert.True(t, image.IsPublic())
	assert.Equal(t, "arm64", image.GetArchitecture())
	assert.Equal(t, "Linux/UNIX", image.GetPlatformDetails())
	assert.Equal(t, "ebs", image.GetRootDeviceType())
	assert.True(t, time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC).Equal(image.GetLastLaunchedTime()))
	assert.True(t, time.Unix(0, 0).Equal(image.GetDeprecationTime()))

	empty := NewImage(mockClient{}, types.Image{ImageId: aws.String("ami-2")})
	assert.Equal(t, []string{}, empty.GetSnapshotIds())
	assert.Equal(t, "-", empty.GetName())
	assert.True(t, time.Unix(0, 0).Equal(empty.GetCreatedAt()))
}

func TestLaunchTemplateFields(t *testing.T) {
	lt := NewLaunchTemplate(mockClient{}, types.LaunchTemplate{
		LaunchTemplateId:     aws.String("lt-1"),
		LaunchTemplateName:   aws.String("eks-nodes"),
		DefaultVersionNumber: aws.Int64(3),
		LatestVersionNumber:  aws.Int64(5),
		CreatedBy:            aws.String("arn:aws:iam::123456789012:role/eks"),
		Operator:             &types.OperatorResponse{Managed: aws.Bool(true), Principal: aws.String("eks.amazonaws.com")},
	})

	assert.Equal(t, "eks-nodes", lt.GetLaunchTemplateName())
	assert.Equal(t, int64(3), lt.GetDefaultVersionNumber())
	assert.Equal(t, int64(5), lt.GetLatestVersionNumber())
	assert.Equal(t, "arn:aws:iam::123456789012:role/eks", lt.GetCreatedBy())
	assert.True(t, lt.IsManaged())

	assert.False(t, NewLaunchTemplate(mockClient{}, types.LaunchTemplate{LaunchTemplateId: aws.String("lt-2")}).IsManaged())
}

// gobRoundTrip encodes a concrete value, so it needs no gob.Register; it checks that the
// embedded SDK structs are gob-encodable, which the cache handlers rely on.
func gobRoundTrip[T any](t *testing.T, value T) T {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(value))

	var decoded T
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))

	return decoded
}

func TestEc2NetworkingGobRoundTrip(t *testing.T) {
	client := mockClient{}

	nat := gobRoundTrip(t, NewNatGateway(client, types.NatGateway{
		NatGatewayId:         aws.String("nat-1"),
		CreateTime:           aws.Time(networkingCreated),
		ConnectivityType:     types.ConnectivityTypePublic,
		NatGatewayAddresses:  []types.NatGatewayAddress{{PublicIp: aws.String("52.1.2.3"), Status: types.NatGatewayAddressStatusSucceeded}},
		ProvisionedBandwidth: &types.ProvisionedBandwidth{Status: aws.String("ok")},
		AttachedAppliances:   []types.NatGatewayAttachedAppliance{{ApplianceArn: aws.String("arn:aws:x")}},
		Tags:                 nameTag("egress"),
	}))
	assert.Equal(t, []string{"52.1.2.3"}, nat.GetPublicIps())
	assert.Equal(t, "egress", nat.GetName())

	igw := gobRoundTrip(t, NewInternetGateway(client, types.InternetGateway{
		InternetGatewayId: aws.String("igw-1"),
		Attachments:       []types.InternetGatewayAttachment{{VpcId: aws.String("vpc-1")}},
	}))
	assert.Equal(t, []string{"vpc-1"}, igw.GetAttachedVpcIds())

	tgw := gobRoundTrip(t, NewTransitGateway(client, types.TransitGateway{
		TransitGatewayId:  aws.String("tgw-1"),
		TransitGatewayArn: aws.String("arn:aws:ec2:eu-west-1:123456789012:transit-gateway/tgw-1"),
		Options: &types.TransitGatewayOptions{
			AmazonSideAsn:     aws.Int64(64512),
			EncryptionSupport: &types.EncryptionSupport{EncryptionState: types.EncryptionStateValueEnabled},
		},
	}))
	assert.Equal(t, int64(64512), tgw.GetAmazonSideAsn())
	assert.Equal(t, "arn:aws:ec2:eu-west-1:123456789012:transit-gateway/tgw-1", tgw.GetArn())

	attachment := gobRoundTrip(t, NewTransitGatewayAttachment(client, types.TransitGatewayAttachment{
		TransitGatewayAttachmentId: aws.String("tgw-attach-1"),
		ResourceType:               types.TransitGatewayAttachmentResourceTypeVpc,
		Association:                &types.TransitGatewayAttachmentAssociation{TransitGatewayRouteTableId: aws.String("tgw-rtb-1")},
	}))
	assert.Equal(t, "tgw-rtb-1", attachment.GetRouteTableId())
	assert.Equal(t, "vpc", attachment.GetResourceType())

	peering := gobRoundTrip(t, NewVpcPeeringConnection(client, types.VpcPeeringConnection{
		VpcPeeringConnectionId: aws.String("pcx-1"),
		RequesterVpcInfo: &types.VpcPeeringConnectionVpcInfo{
			VpcId:          aws.String("vpc-a"),
			CidrBlockSet:   []types.CidrBlock{{CidrBlock: aws.String("10.0.0.0/16")}},
			PeeringOptions: &types.VpcPeeringConnectionOptionsDescription{AllowDnsResolutionFromRemoteVpc: aws.Bool(true)},
		},
		Status: &types.VpcPeeringConnectionStateReason{Code: types.VpcPeeringConnectionStateReasonCodeActive},
	}))
	assert.Equal(t, "vpc-a", peering.GetRequesterVpcId())
	assert.Equal(t, "active", peering.GetStatusCode())

	image := gobRoundTrip(t, NewImage(client, types.Image{
		ImageId:             aws.String("ami-1"),
		CreationDate:        aws.String("2024-03-01T12:30:00.000Z"),
		BlockDeviceMappings: []types.BlockDeviceMapping{{Ebs: &types.EbsBlockDevice{SnapshotId: aws.String("snap-1")}}},
		ProductCodes:        []types.ProductCode{{ProductCodeId: aws.String("pc-1")}},
		StateReason:         &types.StateReason{Code: aws.String("ok")},
	}))
	assert.Equal(t, []string{"snap-1"}, image.GetSnapshotIds())
	assert.True(t, networkingCreated.Equal(image.GetCreatedAt()))

	lt := gobRoundTrip(t, NewLaunchTemplate(client, types.LaunchTemplate{
		LaunchTemplateId: aws.String("lt-1"),
		Operator:         &types.OperatorResponse{Managed: aws.Bool(true)},
	}))
	assert.True(t, lt.IsManaged())
}
