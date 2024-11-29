package s3

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
	S3(optFns ...func(o *s3.Options)) *s3.Client
}

type S3Repository struct {
	ctx    context.Context
	client AwsClient
}

func NewS3Repository(ctx context.Context, client AwsClient) *S3Repository {
	repo := &S3Repository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *S3Repository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *S3Repository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *S3Repository) ListBucketsAll() ([]Bucket, error) {
	return r.ListBucketsByInput(&s3.ListBucketsInput{})
}

func (r *S3Repository) ListBucketsByInput(query *s3.ListBucketsInput) ([]Bucket, error) {
	log.Trace().
		Str("region", r.client.GetRegion().String()).
		Msg("[S3Repository.ListBucketsByInput] list s3 buckets")

	start := time.Now()
	var buckets []Bucket

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ListBuckets", cfg.ResourceTypeBucket)).Inc()
	}

	resp, err := r.client.S3().ListBuckets(r.ctx, query)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ListBuckets", cfg.ResourceTypeBucket)).Inc()
		}

		return buckets, errors.New(err)
	}

	for _, v := range resp.Buckets {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.
				With(r.promLabels("GetBucketLocation", cfg.ResourceTypeBucket)).
				Inc()
		}

		locationOutput, err := getS3BucketLocation(r.ctx, r.client.S3(), aws.ToString(v.Name))
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetBucketLocation", cfg.ResourceTypeBucket)).Inc()
			}

			return buckets, errors.New(err)
		}

		if string(locationOutput.LocationConstraint) != r.client.GetRegion().String() {
			continue
		}

		tags, _ := r.GetTags(v)
		bucket := NewBucket(r.client, v, tags)
		buckets = append(buckets, bucket)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListBuckets", cfg.ResourceTypeBucket)).
			Add(float64(len(buckets)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListBuckets", cfg.ResourceTypeBucket)).
			Observe(time.Since(start).Seconds())
	}

	return buckets, nil
}

func (r *S3Repository) GetTags(bucket types.Bucket) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetBucketTagging", cfg.ResourceTypeBucket)).
			Inc()
	}

	tagOutput, err := r.client.S3().GetBucketTagging(r.ctx, &s3.GetBucketTaggingInput{Bucket: bucket.Name})
	if err != nil {
		log.Debug().Str("bucket", aws.ToString(bucket.Name)).Err(err).Msg("failed to fetch s3 tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.TagSet, nil
}

var s3RegionCacheInstance *s3RegionCache

func getS3BucketLocation(ctx context.Context, client *s3.Client, bucket string) (*s3.GetBucketLocationOutput, error) {
	if s3RegionCacheInstance == nil {
		s3RegionCacheInstance = &s3RegionCache{
			data: map[string]*s3.GetBucketLocationOutput{},
		}
	}

	return s3RegionCacheInstance.getLocation(ctx, client, bucket)
}

type s3RegionCache struct {
	mu   sync.RWMutex
	data map[string]*s3.GetBucketLocationOutput
}

func (c *s3RegionCache) getLocation(ctx context.Context, client *s3.Client, bucket string) (*s3.GetBucketLocationOutput, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if locationOutput, ok := c.data[bucket]; ok {
		return locationOutput, nil
	}

	locationOutput, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: &bucket})
	if err != nil {
		return nil, err
	}

	// write to cache
	c.data[bucket] = locationOutput

	return locationOutput, nil
}
