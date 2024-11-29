package cloudcontrol

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/rs/zerolog/log"
)

func (r *CloudControlRepository) ListVolumesAll() ([]Volume, error) {
	var volumes []Volume

	query := &cc.ListResourcesInput{
		TypeName: aws.String(string(cfg.ResourceTypeVolume)),
	}

	ccResources, err := r.FindResources(query)
	if err != nil {
		return volumes, errors.New(err)
	}

	for _, ccResource := range ccResources {
		attributes, tags, err := ParseAttributes(ccResource)
		if err != nil {
			log.Err(err).Msg("[CloudControlRepository.ListVolumesAll] failed parsing cloudcontrol ec2 volume attributes")
			continue
		}

		volume := NewVolume(r.client, ccResource, attributes, tags)
		volumes = append(volumes, volume)
	}

	return volumes, nil
}
