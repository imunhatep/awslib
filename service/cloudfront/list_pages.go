package cloudfront

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

// listPages drives one of CloudFront's Marker-paginated List calls to the end.
//
// Most of the configuration-object lists (cache policies, functions, key groups,
// ...) have no SDK paginator, and the ones that do differ from those that do not
// only in the output field names, so every lister goes through this one loop.
// fetch receives the marker to send and returns the page's items and its
// NextMarker; an empty NextMarker ends the listing.
//
// A repeated marker is an error rather than an endless loop: a stub or a proxy
// that ignores the marker would otherwise spin forever on page one.
func listPages[T any](
	r *CloudFrontRepository,
	method string,
	resourceType cfg.ResourceType,
	marker *string,
	fetch func(marker *string) ([]T, *string, error),
) ([]T, error) {
	var out []T

	seen := map[string]bool{}

	for {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels(method, resourceType)).Inc()
		}

		items, next, err := fetch(marker)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels(method, resourceType)).Inc()
			}

			return nil, classifyErr(err)
		}

		out = append(out, items...)

		nextMarker := aws.ToString(next)
		if nextMarker == "" {
			return out, nil
		}

		if seen[nextMarker] {
			return nil, errors.Errorf("%s: pagination marker %q was returned twice", method, nextMarker)
		}

		seen[nextMarker] = true
		marker = aws.String(nextMarker)
	}
}

// observeList records the fetched-resource count and the duration of a
// repository-level listing, under the API method and the repository method
// respectively, matching the labels the distribution listing uses.
func (r *CloudFrontRepository) observeList(apiMethod, repoMethod string, resourceType cfg.ResourceType, count int, start time.Time) {
	if !metrics.AwsMetricsEnabled {
		return
	}

	metrics.AwsApiResourcesFetched.
		With(r.promLabels(apiMethod, resourceType)).
		Add(float64(count))

	metrics.AwsRepoCallDuration.
		With(r.promLabels(repoMethod, resourceType)).
		Observe(time.Since(start).Seconds())
}

// epoch is the CreatedAt of entities whose list response reports no creation
// time. A last-modified time passed off as a creation time would be wrong, so
// these follow the other entities without one.
var epoch = time.Unix(0, 0)

// createdOrEpoch returns the creation time when the API reported one.
func createdOrEpoch(t *time.Time) time.Time {
	if t == nil {
		return epoch
	}

	return *t
}
