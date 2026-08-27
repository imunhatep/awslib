package s3

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/provider/v3/clients/s3"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/awslib/service/resourcegroupstagging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

// regionUsEast1 is spelled out because BucketLocationConstraint has no constant
// for it: GetBucketLocation reports us-east-1 as an *empty* LocationConstraint.
const regionUsEast1 = "us-east-1"

// regionEuWest1 pairs with the legacy "EU" LocationConstraint, which names
// eu-west-1 and is still returned for buckets created under that spelling.
const regionEuWest1 = "eu-west-1"

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type S3Repository struct {
	ctx    context.Context
	client *v3.Client
}

func NewS3Repository(ctx context.Context, client *v3.Client) *S3Repository {
	repo := &S3Repository{
		ctx:    ctx,
		client: client,
	}

	return repo
}

func (r *S3Repository) s3Client(optFns ...func(*awss3.Options)) *awss3.Client {
	return s3.GetClient(r.client, optFns...)
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
	return r.ListBucketsByInput(&awss3.ListBucketsInput{})
}

// ListBucketsByInput returns the buckets that live in this client's region.
//
// ListBuckets is an account-wide call — it returns every bucket the credentials
// can see, in every region — so the region scoping has to happen somewhere. It
// used to happen here, with a GetBucketLocation per bucket per region swept.
// S3 can do it instead, via the bucket-region request parameter, which turns
// that N+1 fan-out into one (paginated) call.
//
// Pagination is not optional with that parameter set: S3 applies a default page
// size of 10,000 buckets and returns a continuation token, so an account past
// that many buckets silently lost the remainder without a paginator.
//
// The BucketRegion that S3 reports per bucket is still checked, because a
// filter the endpoint ignored would otherwise hand back the whole account as if
// it were this region. S3 populates that field whenever the request carries at
// least one parameter, so the check costs nothing; only when it comes back empty
// — an S3-compatible endpoint implementing neither half — does this fall back to
// the old per-bucket GetBucketLocation.
//
// Tags are resolved for the whole region in one Resource Groups Tagging API
// sweep rather than one GetBucketTagging per bucket, which is why the buckets
// are collected first and turned into entities afterwards. See bucketTags.
func (r *S3Repository) ListBucketsByInput(query *awss3.ListBucketsInput) ([]Bucket, error) {
	region := r.client.GetRegion().String()

	log.Trace().
		Str("region", region).
		Msg("[S3Repository.ListBucketsByInput] list s3 buckets")

	input, err := scopeToRegion(query, region)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	var found []types.Bucket

	s3c := r.s3Client()
	p := awss3.NewListBucketsPaginator(s3c, input)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListBuckets", cfg.ResourceTypeBucket)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("ListBuckets", cfg.ResourceTypeBucket)).Inc()
			}

			// Partial results are returned with the error rather than discarded,
			// as they always were: one failed page should not cost the caller the
			// pages that succeeded.
			return r.newBuckets(found), errors.New(err)
		}

		for _, v := range resp.Buckets {
			inRegion, err := r.bucketInRegion(s3c, v)
			if err != nil {
				return r.newBuckets(found), errors.New(err)
			}

			if inRegion {
				found = append(found, v)
			}
		}
	}

	buckets := r.newBuckets(found)

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

// bucketInRegion reports whether a listed bucket belongs to this client's
// region, trusting the region S3 reported and paying for a GetBucketLocation
// only when it reported none.
func (r *S3Repository) bucketInRegion(client *awss3.Client, bucket types.Bucket) (bool, error) {
	region := r.client.GetRegion().String()

	if reported := aws.ToString(bucket.BucketRegion); reported != "" {
		return reported == region, nil
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetBucketLocation", cfg.ResourceTypeBucket)).
			Inc()
	}

	locationOutput, err := getS3BucketLocation(r.ctx, client, aws.ToString(bucket.Name))
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetBucketLocation", cfg.ResourceTypeBucket)).Inc()
		}

		return false, errors.New(err)
	}

	return bucketLocationRegion(string(locationOutput.LocationConstraint)) == region, nil
}

// scopeToRegion returns a copy of query restricted to region via the
// bucket-region request parameter.
//
// The copy matters: the input belongs to the caller, and the generated cached
// wrapper has already derived its cache key from it.
func scopeToRegion(query *awss3.ListBucketsInput, region string) (*awss3.ListBucketsInput, error) {
	input := *query

	switch bucketRegion := aws.ToString(input.BucketRegion); bucketRegion {
	case "":
		input.BucketRegion = aws.String(region)
	case region:
		// caller already scoped the query to this client's region
	default:
		// S3 rejects a bucket-region that disagrees with the endpoint's region.
		// Saying so beats forwarding a generic SDK error, since the fix is to ask
		// the pool for a client in that region instead.
		return nil, errors.Errorf(
			"s3 bucket region %q does not match client region %q, use a client for that region",
			bucketRegion, region,
		)
	}

	return &input, nil
}

// bucketLocationRegion normalizes a GetBucketLocation LocationConstraint into a
// region name.
//
// Two values do not name their own region. An empty constraint means us-east-1,
// not "no region" — comparing the raw value against the client region hid every
// us-east-1 bucket. "EU" is the legacy spelling of eu-west-1, still returned for
// buckets created under it.
func bucketLocationRegion(constraint string) string {
	switch constraint {
	case "":
		return regionUsEast1
	case string(types.BucketLocationConstraintEu):
		return regionEuWest1
	}

	return constraint
}

// newBuckets turns listed buckets into entities, resolving every bucket's tags
// in one pass. Returns nil for an empty input so a caller cannot tell an
// unwritten result from an empty one.
func (r *S3Repository) newBuckets(found []types.Bucket) []Bucket {
	if len(found) == 0 {
		return nil
	}

	tags := r.bucketTags(found)

	buckets := make([]Bucket, 0, len(found))
	for _, v := range found {
		buckets = append(buckets, NewBucket(r.client, v, tags[aws.ToString(v.Name)]))
	}

	return buckets
}

// bucketTags returns the tags of the given buckets, keyed by bucket name.
//
// One Resource Groups Tagging API sweep answers for every bucket in the region,
// where GetBucketTagging answers for one — the difference between a few calls
// and one per bucket, on every region of every account swept.
//
// A bucket missing from the sweep has no tags. That is an answer, not a gap:
// the API returns every resource that is currently tagged, so a bucket with tags
// is in the result by definition. Confirming absences per bucket would give back
// the entire fan-out in any account where most buckets are untagged.
//
// The sweep *failing* is a different thing, and the only reason for a fallback.
// It most likely means the role lacks tag:GetResources, a permission the
// per-bucket path never needed, so falling back keeps tags flowing for a
// consumer that has not updated its IAM yet — rather than reporting every bucket
// as untagged on the strength of an answer that never arrived.
func (r *S3Repository) bucketTags(buckets []types.Bucket) map[string][]types.Tag {
	tagRepo := resourcegroupstagging.NewResourceGroupsTaggingRepository(r.ctx, r.client)

	mappings, err := tagRepo.GetResourceTagsByType(cfg.ResourceTypeBucket)
	if err != nil {
		log.Warn().Err(err).
			Str("accountID", r.client.GetAccountID().String()).
			Str("region", r.client.GetRegion().String()).
			Int("buckets", len(buckets)).
			Msg("[S3Repository.bucketTags] resource groups tagging sweep failed, falling back to GetBucketTagging per bucket")

		return r.bucketTagsPerBucket(buckets)
	}

	log.Debug().
		Str("accountID", r.client.GetAccountID().String()).
		Str("region", r.client.GetRegion().String()).
		Int("buckets", len(buckets)).
		Msgf("[S3Repository.bucketTags] tagged buckets reported by the tagging api: %d", len(mappings))

	return joinBucketTags(buckets, resourcegroupstagging.NewTagIndex(mappings))
}

// bucketTagsPerBucket is the pre-tagging-api path, kept as the fallback: one
// GetBucketTagging per bucket.
//
// The per-bucket error stays discarded, as it always was, because S3 answers an
// untagged bucket with a NoSuchTagSet error rather than an empty tag set — the
// common case arrives as a failure and must not be reported as one.
func (r *S3Repository) bucketTagsPerBucket(buckets []types.Bucket) map[string][]types.Tag {
	tags := make(map[string][]types.Tag, len(buckets))

	for _, bucket := range buckets {
		bucketTags, _ := r.GetTags(bucket)
		tags[aws.ToString(bucket.Name)] = bucketTags
	}

	return tags
}

// joinBucketTags matches buckets to their tags by name.
//
// The join is on the name rather than the ARN because the two ARNs never agree
// as strings: the tagging API reports arn:aws:s3:::<name>, with no region and no
// account, while Bucket's own ARN is built with both. The name is the only
// identifying segment an S3 bucket ARN has, and bucket names are globally
// unique, so the tag index's identifier lookup cannot collide here — nor does
// this need to know the partition, which building an ARN would.
func joinBucketTags(buckets []types.Bucket, index *resourcegroupstagging.TagIndex) map[string][]types.Tag {
	tags := make(map[string][]types.Tag, len(buckets))

	for _, bucket := range buckets {
		name := aws.ToString(bucket.Name)
		tags[name] = tagsFromMap(index.LookupId(name))
	}

	return tags
}

// tagsFromMap converts a tag map into the SDK's tag list, sorted by key.
//
// The sort is not cosmetic: Bucket is cached with its Tags slice, and Go's map
// iteration order would otherwise make a freshly fetched bucket differ from the
// cached one that encodes the same tags.
func tagsFromMap(tags map[string]string) []types.Tag {
	list := make([]types.Tag, 0, len(tags))
	if len(tags) == 0 {
		return list
	}

	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		list = append(list, types.Tag{Key: aws.String(key), Value: aws.String(tags[key])})
	}

	return list
}

func (r *S3Repository) GetTags(bucket types.Bucket) ([]types.Tag, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("GetBucketTagging", cfg.ResourceTypeBucket)).
			Inc()
	}

	tagOutput, err := r.s3Client().GetBucketTagging(r.ctx, &awss3.GetBucketTaggingInput{Bucket: bucket.Name})
	if err != nil {
		log.Debug().Str("bucket", aws.ToString(bucket.Name)).Err(err).Msg("failed to fetch s3 tags")
		return []types.Tag{}, errors.New(err)
	}

	return tagOutput.TagSet, nil
}

var s3RegionCacheInstance *s3RegionCache

func getS3BucketLocation(ctx context.Context, client *awss3.Client, bucket string) (*awss3.GetBucketLocationOutput, error) {
	if s3RegionCacheInstance == nil {
		s3RegionCacheInstance = &s3RegionCache{
			data: map[string]*awss3.GetBucketLocationOutput{},
		}
	}

	return s3RegionCacheInstance.getLocation(ctx, client, bucket)
}

type s3RegionCache struct {
	mu   sync.RWMutex
	data map[string]*awss3.GetBucketLocationOutput
}

func (c *s3RegionCache) getLocation(ctx context.Context, client *awss3.Client, bucket string) (*awss3.GetBucketLocationOutput, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if locationOutput, ok := c.data[bucket]; ok {
		return locationOutput, nil
	}

	locationOutput, err := client.GetBucketLocation(ctx, &awss3.GetBucketLocationInput{Bucket: &bucket})
	if err != nil {
		return nil, err
	}

	// write to cache
	c.data[bucket] = locationOutput

	return locationOutput, nil
}
