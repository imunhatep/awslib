package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/gocollection/dict"
)

func BuildCreateTagsInput(tags map[string]string, resources ...service.EntityInterface) *ec2.CreateTagsInput {
	// Filter out resources that do not have the tag key in the tags
	resourceTagMissing := func(resourceTags map[string]string) bool {
		for tag, value := range tags {
			if _, ok := resourceTags[tag]; !ok {
				continue
			}

			// compare tag value with
			if resourceTags[tag] == value {
				return false
			}
		}

		return true
	}

	resourceIds := []string{}
	for _, resource := range resources {
		if resourceTagMissing(resource.GetTags()) {
			resourceIds = append(resourceIds, resource.GetId())
		}
	}

	if len(resourceIds) == 0 {
		return nil
	}

	tagsInput := &ec2.CreateTagsInput{
		Resources: resourceIds,
		Tags:      TagMapToTags(tags),
	}

	return tagsInput
}

func BuildDeleteTagsInput(tags map[string]string, resources ...service.EntityInterface) *ec2.DeleteTagsInput {
	removeTags := map[string]string{}
	resourceIds := map[string]string{}
	for _, resource := range resources {
		resourceTags := resource.GetTags()

		for tag, value := range tags {
			// filter out resources that do not have the tag key in the tags
			if _, ok := resourceTags[tag]; !ok {
				continue
			}

			// either tag value matches or value is empty
			if value == "" || resourceTags[tag] == value {
				resourceIds[resource.GetId()] = resource.GetId()
				removeTags[tag] = resourceTags[tag]
			}
		}
	}

	if len(resourceIds) == 0 {
		return nil
	}

	input := &ec2.DeleteTagsInput{
		Tags:      TagMapToTags(removeTags),
		Resources: dict.Keys(resourceIds),
	}

	return input
}

func TagMapToTags(tags map[string]string) []types.Tag {
	tagsToAdd := []types.Tag{}

	for key, value := range tags {
		tagsToAdd = append(
			tagsToAdd,
			types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			},
		)
	}

	return tagsToAdd
}
