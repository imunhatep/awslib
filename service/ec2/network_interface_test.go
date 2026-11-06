package ec2

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nlbInterface is the shape DescribeNetworkInterfaces returns for one AZ of an
// internet-facing Network Load Balancer without Elastic IPs: requester-managed, named only
// by its description, and carrying an auto-assigned public address that
// DescribeLoadBalancers does not report anywhere.
func nlbInterface() types.NetworkInterface {
	return types.NetworkInterface{
		NetworkInterfaceId: aws.String("eni-0a1b2c"),
		Description:        aws.String("ELB net/ingress/818502e1be7cef4d"),
		InterfaceType:      types.NetworkInterfaceTypeNetworkLoadBalancer,
		Status:             types.NetworkInterfaceStatusInUse,
		RequesterManaged:   aws.Bool(true),
		PrivateIpAddress:   aws.String("10.17.204.134"),
		Association: &types.NetworkInterfaceAssociation{
			PublicIp: aws.String("15.240.22.35"),
		},
		PrivateIpAddresses: []types.NetworkInterfacePrivateIpAddress{{
			Primary:          aws.Bool(true),
			PrivateIpAddress: aws.String("10.17.204.134"),
			Association:      &types.NetworkInterfaceAssociation{PublicIp: aws.String("15.240.22.35")},
		}},
		Attachment: &types.NetworkInterfaceAttachment{
			AttachTime: aws.Time(time.Date(2026, 9, 9, 8, 6, 53, 0, time.UTC)),
		},
	}
}

func TestNetworkInterfaceIdentity(t *testing.T) {
	eni := NewNetworkInterface(mockClient{}, nlbInterface())

	assert.Equal(t, "eni-0a1b2c", eni.GetId())
	assert.Equal(t, "arn:aws:ec2:eu-west-1:123456789012:network-interface/eni-0a1b2c", eni.GetArn())
	assert.Equal(t, cfg.ResourceTypeNetworkInterface, eni.GetType())
	assert.Equal(t, "ELB net/ingress/818502e1be7cef4d", eni.GetName(), "an untagged ENI is named by its description")
	assert.Equal(t, time.Date(2026, 9, 9, 8, 6, 53, 0, time.UTC), eni.GetCreatedAt())

	tagged := nlbInterface()
	tagged.TagSet = []types.Tag{{Key: aws.String("Name"), Value: aws.String("frontend-a")}}
	assert.Equal(t, "frontend-a", NewNetworkInterface(mockClient{}, tagged).GetName())

	assert.Equal(t, "-", NewNetworkInterface(mockClient{}, types.NetworkInterface{NetworkInterfaceId: aws.String("eni-1")}).GetName())
}

// The auto-assigned public address is the reason this entity exists: it is not an Elastic IP,
// so DescribeAddresses does not list it either.
func TestNetworkInterfaceAutoAssignedPublicIp(t *testing.T) {
	eni := NewNetworkInterface(mockClient{}, nlbInterface())

	assert.Equal(t, "10.17.204.134", eni.GetPrivateIpAddress())
	assert.Equal(t, "15.240.22.35", eni.GetPublicIp())
	assert.False(t, eni.IsElasticIp())
	assert.True(t, eni.IsRequesterManaged())
	assert.Equal(t, types.NetworkInterfaceTypeNetworkLoadBalancer, eni.GetInterfaceType())
}

func TestNetworkInterfaceMultipleAddresses(t *testing.T) {
	eni := NewNetworkInterface(mockClient{}, types.NetworkInterface{
		NetworkInterfaceId: aws.String("eni-1"),
		PrivateIpAddress:   aws.String("10.0.0.10"),
		Association: &types.NetworkInterfaceAssociation{
			PublicIp:     aws.String("52.1.2.3"),
			AllocationId: aws.String("eipalloc-1"),
		},
		Attachment: &types.NetworkInterfaceAttachment{InstanceId: aws.String("i-1")},
		// Secondary listed first: the API does not promise primary-first ordering.
		PrivateIpAddresses: []types.NetworkInterfacePrivateIpAddress{
			{PrivateIpAddress: aws.String("10.0.0.11"), Association: &types.NetworkInterfaceAssociation{PublicIp: aws.String("52.1.2.4")}},
			{PrivateIpAddress: aws.String("10.0.0.12")},
			{Primary: aws.Bool(true), PrivateIpAddress: aws.String("10.0.0.10"), Association: &types.NetworkInterfaceAssociation{PublicIp: aws.String("52.1.2.3")}},
		},
	})

	assert.Equal(t, []string{"10.0.0.10", "10.0.0.11", "10.0.0.12"}, eni.GetPrivateIpAddresses())
	assert.Equal(t, []string{"52.1.2.3", "52.1.2.4"}, eni.GetPublicIps())
	assert.True(t, eni.IsElasticIp())
	assert.Equal(t, "i-1", eni.GetInstanceId())
}

func TestNetworkInterfaceWithoutPublicAddress(t *testing.T) {
	eni := NewNetworkInterface(mockClient{}, types.NetworkInterface{
		NetworkInterfaceId: aws.String("eni-1"),
		PrivateIpAddress:   aws.String("10.0.0.10"),
	})

	assert.Equal(t, "", eni.GetPublicIp())
	assert.Empty(t, eni.GetPublicIps())
	assert.False(t, eni.IsElasticIp())
	assert.Equal(t, "", eni.GetInstanceId())
	assert.Equal(t, time.Unix(0, 0), eni.GetCreatedAt())
}

func TestNetworkInterfaceGobRoundTrip(t *testing.T) {
	original := []service.ResourceInterface{NewNetworkInterface(mockClient{}, nlbInterface())}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(original))

	var decoded []service.ResourceInterface
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))
	require.Len(t, decoded, 1)

	eni, ok := decoded[0].(NetworkInterface)
	require.True(t, ok)
	assert.Equal(t, "15.240.22.35", eni.GetPublicIp())
	assert.Equal(t, "10.17.204.134", eni.GetPrivateIpAddress())
}
