package ec2

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockClient struct{}

func (mockClient) GetRegion() ptypes.AwsRegion       { return ptypes.AwsRegion("eu-west-1") }
func (mockClient) GetAccountID() ptypes.AwsAccountID { return ptypes.AwsAccountID("123456789012") }

func TestVpcNetworkResourceInterface(t *testing.T) {
	client := mockClient{}

	tests := []struct {
		name     string
		resource service.ResourceInterface
		id       string
		arn      string
		rType    cfg.ResourceType
		rName    string
	}{
		{
			name: "subnet uses the ARN returned by the API",
			resource: NewSubnet(client, types.Subnet{
				SubnetId:  aws.String("subnet-0a1b2c"),
				SubnetArn: aws.String("arn:aws:ec2:eu-west-1:123456789012:subnet/subnet-0a1b2c"),
				Tags:      []types.Tag{{Key: aws.String("Name"), Value: aws.String("private-a")}},
			}),
			id:    "subnet-0a1b2c",
			arn:   "arn:aws:ec2:eu-west-1:123456789012:subnet/subnet-0a1b2c",
			rType: cfg.ResourceTypeSubnet,
			rName: "private-a",
		},
		{
			name:     "subnet without an API ARN falls back to a synthesized one",
			resource: NewSubnet(client, types.Subnet{SubnetId: aws.String("subnet-0a1b2c")}),
			id:       "subnet-0a1b2c",
			arn:      "arn:aws:ec2:eu-west-1:123456789012:subnet/subnet-0a1b2c",
			rType:    cfg.ResourceTypeSubnet,
			rName:    "-",
		},
		{
			name: "security group falls back to the group name",
			resource: NewSecurityGroup(client, types.SecurityGroup{
				GroupId:   aws.String("sg-0a1b2c"),
				GroupName: aws.String("web-tier"),
			}),
			id:    "sg-0a1b2c",
			arn:   "arn:aws:ec2:eu-west-1:123456789012:security-group/sg-0a1b2c",
			rType: cfg.ResourceTypeSecurityGroup,
			rName: "web-tier",
		},
		{
			name: "vpc endpoint falls back to the service name",
			resource: NewVpcEndpoint(client, types.VpcEndpoint{
				VpcEndpointId: aws.String("vpce-0a1b2c"),
				ServiceName:   aws.String("com.amazonaws.eu-west-1.s3"),
			}),
			id:    "vpce-0a1b2c",
			arn:   "arn:aws:ec2:eu-west-1:123456789012:vpc-endpoint/vpce-0a1b2c",
			rType: cfg.ResourceTypeVPCEndpoint,
			rName: "com.amazonaws.eu-west-1.s3",
		},
		{
			name:     "route table",
			resource: NewRouteTable(client, types.RouteTable{RouteTableId: aws.String("rtb-0a1b2c")}),
			id:       "rtb-0a1b2c",
			arn:      "arn:aws:ec2:eu-west-1:123456789012:route-table/rtb-0a1b2c",
			rType:    cfg.ResourceTypeRouteTable,
			rName:    "-",
		},
		{
			name: "elastic ip is keyed by allocation id and named by its public address",
			resource: NewAddress(client, types.Address{
				AllocationId:     aws.String("eipalloc-0a1b2c"),
				PublicIp:         aws.String("52.1.2.3"),
				PrivateIpAddress: aws.String("10.0.1.5"),
			}),
			id:    "eipalloc-0a1b2c",
			arn:   "arn:aws:ec2:eu-west-1:123456789012:elastic-ip/eipalloc-0a1b2c",
			rType: cfg.ResourceTypeEip,
			rName: "52.1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.id, tt.resource.GetId())
			assert.Equal(t, tt.arn, tt.resource.GetArn())
			assert.Equal(t, tt.rType, tt.resource.GetType())
			assert.Equal(t, tt.rName, tt.resource.GetName())
			assert.Equal(t, ptypes.AwsAccountID("123456789012"), tt.resource.GetAccountID())
			assert.Equal(t, ptypes.AwsRegion("eu-west-1"), tt.resource.GetRegion())
		})
	}
}

// An EC2-Classic style address has no AllocationId, so the public IP has to identify it.
func TestAddressWithoutAllocationId(t *testing.T) {
	addr := NewAddress(mockClient{}, types.Address{PublicIp: aws.String("52.1.2.3")})

	assert.Equal(t, "52.1.2.3", addr.GetId())
	assert.Equal(t, "arn:aws:ec2:eu-west-1:123456789012:elastic-ip/52.1.2.3", addr.GetArn())
}

func TestAddressAssociation(t *testing.T) {
	unassociated := NewAddress(mockClient{}, types.Address{AllocationId: aws.String("eipalloc-1")})
	assert.False(t, unassociated.IsAssociated())
	assert.Equal(t, "", unassociated.GetPrivateIpAddress())

	associated := NewAddress(mockClient{}, types.Address{
		AllocationId:     aws.String("eipalloc-2"),
		AssociationId:    aws.String("eipassoc-2"),
		PublicIp:         aws.String("52.1.2.3"),
		PrivateIpAddress: aws.String("10.0.1.5"),
	})
	assert.True(t, associated.IsAssociated())
	assert.Equal(t, "52.1.2.3", associated.GetPublicIp())
	assert.Equal(t, "10.0.1.5", associated.GetPrivateIpAddress())
}

func TestRouteTableMainAndSubnets(t *testing.T) {
	rtb := NewRouteTable(mockClient{}, types.RouteTable{
		RouteTableId: aws.String("rtb-0a1b2c"),
		VpcId:        aws.String("vpc-0a1b2c"),
		Associations: []types.RouteTableAssociation{
			{Main: aws.Bool(true)},
			{SubnetId: aws.String("subnet-1")},
			{SubnetId: aws.String("subnet-2")},
		},
	})

	assert.True(t, rtb.IsMain())
	assert.Equal(t, "vpc-0a1b2c", rtb.GetVpcId())
	assert.Equal(t, []string{"subnet-1", "subnet-2"}, rtb.GetSubnetIds())

	standalone := NewRouteTable(mockClient{}, types.RouteTable{RouteTableId: aws.String("rtb-2")})
	assert.False(t, standalone.IsMain())
	assert.Equal(t, []string{}, standalone.GetSubnetIds())
}

func TestSubnetIsPublic(t *testing.T) {
	public := NewSubnet(mockClient{}, types.Subnet{
		SubnetId:            aws.String("subnet-1"),
		MapPublicIpOnLaunch: aws.Bool(true),
	})
	assert.True(t, public.IsPublic())

	private := NewSubnet(mockClient{}, types.Subnet{SubnetId: aws.String("subnet-2")})
	assert.False(t, private.IsPublic())
}

// The cache handlers serialize with encoding/gob, so every entity has to survive a
// round-trip through an interface-typed slice — that is what the public DataCache API stores.
func TestVpcNetworkGobRoundTrip(t *testing.T) {
	client := mockClient{}

	original := []service.ResourceInterface{
		NewSubnet(client, types.Subnet{
			SubnetId:            aws.String("subnet-1"),
			VpcId:               aws.String("vpc-1"),
			CidrBlock:           aws.String("10.0.1.0/24"),
			AvailabilityZone:    aws.String("eu-west-1a"),
			State:               types.SubnetStateAvailable,
			MapPublicIpOnLaunch: aws.Bool(true),
			Tags:                []types.Tag{{Key: aws.String("Name"), Value: aws.String("public-a")}},
		}),
		NewSecurityGroup(client, types.SecurityGroup{
			GroupId:   aws.String("sg-1"),
			GroupName: aws.String("web-tier"),
			VpcId:     aws.String("vpc-1"),
			IpPermissions: []types.IpPermission{{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(443),
				ToPort:     aws.Int32(443),
				IpRanges:   []types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
			}},
		}),
		NewVpcEndpoint(client, types.VpcEndpoint{
			VpcEndpointId:   aws.String("vpce-1"),
			VpcId:           aws.String("vpc-1"),
			ServiceName:     aws.String("com.amazonaws.eu-west-1.s3"),
			VpcEndpointType: types.VpcEndpointTypeGateway,
			State:           types.StateAvailable,
			RouteTableIds:   []string{"rtb-1"},
		}),
		NewRouteTable(client, types.RouteTable{
			RouteTableId: aws.String("rtb-1"),
			VpcId:        aws.String("vpc-1"),
			Routes: []types.Route{{
				DestinationCidrBlock: aws.String("0.0.0.0/0"),
				GatewayId:            aws.String("igw-1"),
			}},
			Associations: []types.RouteTableAssociation{{Main: aws.Bool(true)}},
		}),
		NewAddress(client, types.Address{
			AllocationId:     aws.String("eipalloc-1"),
			PublicIp:         aws.String("52.1.2.3"),
			PrivateIpAddress: aws.String("10.0.1.5"),
			Domain:           types.DomainTypeVpc,
		}),
	}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(original))

	var decoded []service.ResourceInterface
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))

	require.Len(t, decoded, len(original))
	for i, want := range original {
		assert.Equal(t, want.GetId(), decoded[i].GetId())
		assert.Equal(t, want.GetArn(), decoded[i].GetArn())
		assert.Equal(t, want.GetType(), decoded[i].GetType())
		assert.Equal(t, want.GetName(), decoded[i].GetName())
		assert.Equal(t, want.GetTags(), decoded[i].GetTags())
	}

	// Spot-check that the embedded SDK payload survived, not just the abstract resource.
	subnet, ok := decoded[0].(Subnet)
	require.True(t, ok)
	assert.Equal(t, "10.0.1.0/24", subnet.GetCidrBlock())
	assert.True(t, subnet.IsPublic())

	sg, ok := decoded[1].(SecurityGroup)
	require.True(t, ok)
	require.Len(t, sg.GetIngressRules(), 1)
	assert.Equal(t, int32(443), aws.ToInt32(sg.GetIngressRules()[0].FromPort))

	eip, ok := decoded[4].(Address)
	require.True(t, ok)
	assert.Equal(t, "10.0.1.5", eip.GetPrivateIpAddress())
}
