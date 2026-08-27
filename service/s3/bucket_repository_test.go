package s3

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/imunhatep/awslib/service/resourcegroupstagging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScopeToRegionSetsBucketRegion pins the change that removed the
// GetBucketLocation fan-out: the region filter has to reach the request, or
// ListBuckets answers with every bucket in the account.
func TestScopeToRegionSetsBucketRegion(t *testing.T) {
	input, err := scopeToRegion(&awss3.ListBucketsInput{}, "eu-central-1")

	require.NoError(t, err)
	assert.Equal(t, "eu-central-1", aws.ToString(input.BucketRegion))
}

// TestScopeToRegionDoesNotMutateCallerInput matters because the generated cached
// wrapper derives its cache key from the caller's input before handing it over.
// Writing BucketRegion into that struct would leave the caller holding a query
// that no longer matches the key it was cached under.
func TestScopeToRegionDoesNotMutateCallerInput(t *testing.T) {
	query := &awss3.ListBucketsInput{Prefix: aws.String("logs-")}

	input, err := scopeToRegion(query, "eu-central-1")

	require.NoError(t, err)
	assert.Nil(t, query.BucketRegion, "caller's input must be left alone")
	assert.Equal(t, "eu-central-1", aws.ToString(input.BucketRegion))
	assert.Equal(t, "logs-", aws.ToString(input.Prefix), "other fields carry over")
}

func TestScopeToRegionAcceptsMatchingRegion(t *testing.T) {
	query := &awss3.ListBucketsInput{BucketRegion: aws.String("eu-central-1")}

	input, err := scopeToRegion(query, "eu-central-1")

	require.NoError(t, err)
	assert.Equal(t, "eu-central-1", aws.ToString(input.BucketRegion))
}

// TestScopeToRegionRejectsForeignRegion covers the request S3 itself refuses: a
// bucket-region that disagrees with the endpoint's region. Failing here names
// the fix (use a client for that region) instead of surfacing an SDK error from
// three frames deeper.
func TestScopeToRegionRejectsForeignRegion(t *testing.T) {
	query := &awss3.ListBucketsInput{BucketRegion: aws.String("us-west-2")}

	input, err := scopeToRegion(query, "eu-central-1")

	require.Error(t, err)
	assert.Nil(t, input)
	assert.Contains(t, err.Error(), "us-west-2")
	assert.Contains(t, err.Error(), "eu-central-1")
}

// TestBucketLocationRegion pins the two LocationConstraint values that do not
// name their own region. The empty case is a fixed bug, not a hypothetical:
// comparing the raw constraint against the client region meant a us-east-1
// client skipped every one of its own buckets.
func TestBucketLocationRegion(t *testing.T) {
	cases := map[string]string{
		"":                                       "us-east-1",
		string(types.BucketLocationConstraintEu): "eu-west-1",
		string(types.BucketLocationConstraintEuWest1):    "eu-west-1",
		string(types.BucketLocationConstraintUsWest2):    "us-west-2",
		string(types.BucketLocationConstraintEuCentral1): "eu-central-1",
	}

	for constraint, expected := range cases {
		assert.Equal(t, expected, bucketLocationRegion(constraint),
			"region for LocationConstraint %q", constraint)
	}
}

// TestJoinBucketTagsMatchesOnName pins the join key. The tagging API reports
// arn:aws:s3:::<name> while Bucket's own ARN carries a region and account, so
// the two ARNs never match as strings — the name is the only thing they share.
func TestJoinBucketTagsMatchesOnName(t *testing.T) {
	index := resourcegroupstagging.NewTagIndex([]resourcegroupstagging.TagMapping{
		{Arn: "arn:aws:s3:::tagged-bucket", Tags: map[string]string{"env": "prod", "Name": "web"}},
		{Arn: "arn:aws:s3:::other-bucket", Tags: map[string]string{"env": "dev"}},
	})

	buckets := []types.Bucket{
		{Name: aws.String("tagged-bucket")},
		{Name: aws.String("untagged-bucket")},
	}

	tags := joinBucketTags(buckets, index)

	assert.Equal(t, []types.Tag{
		{Key: aws.String("Name"), Value: aws.String("web")},
		{Key: aws.String("env"), Value: aws.String("prod")},
	}, tags["tagged-bucket"])

	// A bucket absent from the sweep has no tags. That is the answer, not a gap
	// to be filled by a per-bucket call.
	assert.Empty(t, tags["untagged-bucket"])
	assert.NotNil(t, tags["untagged-bucket"])
}

// TestJoinBucketTagsWorksInOtherPartitions covers why the join does not build an
// ARN to look up: the partition differs outside commercial AWS, and the name
// carries none of it.
func TestJoinBucketTagsWorksInOtherPartitions(t *testing.T) {
	index := resourcegroupstagging.NewTagIndex([]resourcegroupstagging.TagMapping{
		{Arn: "arn:aws-cn:s3:::cn-bucket", Tags: map[string]string{"env": "prod"}},
	})

	tags := joinBucketTags([]types.Bucket{{Name: aws.String("cn-bucket")}}, index)

	assert.Equal(t, []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}}, tags["cn-bucket"])
}

// TestTagsFromMapIsSortedByKey pins the sort. Bucket is cached with its Tags
// slice, so map iteration order would make a freshly fetched bucket differ from
// a cached one holding the same tags.
func TestTagsFromMapIsSortedByKey(t *testing.T) {
	list := tagsFromMap(map[string]string{"zulu": "3", "alpha": "1", "mike": "2"})

	assert.Equal(t, []types.Tag{
		{Key: aws.String("alpha"), Value: aws.String("1")},
		{Key: aws.String("mike"), Value: aws.String("2")},
		{Key: aws.String("zulu"), Value: aws.String("3")},
	}, list)
}

func TestTagsFromMapEmpty(t *testing.T) {
	assert.Empty(t, tagsFromMap(nil))
	assert.NotNil(t, tagsFromMap(nil), "an empty tag list, not nil, keeps Bucket.GetTags allocation-free")
}
