package cloudcontrol

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/rs/zerolog/log"
)

func (r *CloudControlRepository) ListInstancesAll() ([]Instance, error) {
	query := &cc.ListResourcesInput{
		TypeName: aws.String(string(cfg.ResourceTypeInstance)),
	}

	return r.ListInstancesByInput(query)
}

func (r *CloudControlRepository) ListInstancesByInput(query *cc.ListResourcesInput) ([]Instance, error) {
	var instances []Instance

	ccResources, err := r.FindResources(query)
	if err != nil {
		return instances, errors.New(err)
	}

	for _, ccResource := range ccResources {
		attributes, tags, err := ParseAttributes(ccResource)
		if err != nil {
			log.Err(err).
				Str("id", aws.ToString(ccResource.Identifier)).
				Msg("[CloudControlRepository.ListDbInstancesAll] failed parsing cloudcontrol ec2 instance attributes")

			continue
		}

		instance := NewInstance(r.client, ccResource, attributes, tags)
		instances = append(instances, instance)
	}

	return instances, nil
}
