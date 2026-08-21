package ec2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/helper"
	"github.com/imunhatep/awslib/service"
)

type RouteTable struct {
	service.AbstractResource
	types.RouteTable
}

func NewRouteTable(client AwsClient, routeTable types.RouteTable) RouteTable {
	return RouteTable{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(routeTable.RouteTableId),
			ARN:       helper.BuildArn(client.GetAccountID(), client.GetRegion(), "ec2", "route-table/", routeTable.RouteTableId),
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeRouteTable,
		},
		RouteTable: routeTable,
	}
}

func (e RouteTable) GetName() string {
	if name, ok := e.GetTags()["Name"]; ok {
		return name
	}

	return "-"
}

func (e RouteTable) GetVpcId() string {
	return aws.ToString(e.RouteTable.VpcId)
}

func (e RouteTable) GetRoutes() []types.Route {
	return e.RouteTable.Routes
}

func (e RouteTable) GetAssociations() []types.RouteTableAssociation {
	return e.RouteTable.Associations
}

// IsMain reports whether the route table is the VPC's main route table.
func (e RouteTable) IsMain() bool {
	for _, association := range e.RouteTable.Associations {
		if aws.ToBool(association.Main) {
			return true
		}
	}

	return false
}

// GetSubnetIds returns the subnets explicitly associated with the route table.
func (e RouteTable) GetSubnetIds() []string {
	subnetIds := []string{}
	for _, association := range e.RouteTable.Associations {
		if association.SubnetId != nil {
			subnetIds = append(subnetIds, aws.ToString(association.SubnetId))
		}
	}

	return subnetIds
}

func (e RouteTable) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.RouteTable.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e RouteTable) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}
