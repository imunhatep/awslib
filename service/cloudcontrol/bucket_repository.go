package cloudcontrol

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/rs/zerolog/log"
)

func (r *CloudControlRepository) ListBucketsAll() ([]Bucket, error) {
	query := &cc.ListResourcesInput{
		TypeName: aws.String(string(cfg.ResourceTypeBucket)),
	}

	return r.ListBucketsByInput(query)
}

func (r *CloudControlRepository) ListBucketsByInput(query *cc.ListResourcesInput) ([]Bucket, error) {
	var buckets []Bucket

	ccResources, err := r.FindResources(query)
	if err != nil {
		return buckets, errors.New(err)
	}

	for _, ccId := range ccResources {
		ccResource, err := r.DescribeResource(cfg.ResourceTypeBucket, ccId.Identifier)
		if err != nil {
			log.Err(err).Msg("[CloudControlRepository.ListBucketsAll] failed fetching resource details")
			continue
		}

		attributes, tags, err := ParseAttributes(*ccResource.ResourceDescription)
		if err != nil {
			log.Err(err).Msg("[CloudControlRepository.ListBucketsAll] failed parsing cloudcontrol s3 bucket attributes")
			continue
		}

		bucket := NewBucket(r.client, *ccResource.ResourceDescription, attributes, tags)
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}
