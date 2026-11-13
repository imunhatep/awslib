// Package wafv2 reads AWS WAF (v2) web ACLs, rule groups, IP sets and regex
// pattern sets, the resources a web ACL protects, sampled requests, and the
// CloudWatch request counts WAF publishes per rule.
//
// Every WAFv2 object has a scope, and the scope decides the endpoint:
//
//   - REGIONAL objects protect regional resources (ALB, API Gateway, AppSync,
//     Cognito, App Runner, Verified Access, AgentCore gateways) and live in the
//     region of the resource they protect.
//   - CLOUDFRONT objects protect CloudFront distributions (and Amplify apps). They
//     are global, but the API serves them only from us-east-1, and their ARNs
//     carry us-east-1 with a "global/" resource prefix.
//
// The listing methods therefore read REGIONAL objects from the client's region
// and add the CLOUDFRONT ones only when the client is in us-east-1 (see
// Scopes). A sweep over every region thus sees each CloudFront-scoped object
// exactly once, attributed to us-east-1, which is also what its ARN says. A
// method given an explicit CLOUDFRONT scope pins the call to us-east-1 whatever
// the client's region, so a single lookup does not depend on which regional
// client happened to be at hand.
package wafv2

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awswaf "github.com/aws/aws-sdk-go-v2/service/wafv2"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	wafclient "github.com/imunhatep/awslib/provider/v3/clients/wafv2"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/prometheus/client_golang/prometheus"
)

// CloudFrontRegion is the only region that serves CLOUDFRONT-scoped objects.
const CloudFrontRegion = ptypes.DefaultAwsRegion

const (
	// listPageSize is the largest page the WAFv2 List* calls accept.
	listPageSize int32 = 100

	// detailFetchConcurrency and detailFetchMaxAttempts shape the per-object Get
	// and ListTagsForResource calls a listing needs: the List* responses carry
	// neither rules nor addresses nor tags. WAFv2's control plane throttles at
	// low single-digit TPS per account for some operations, so the reads are kept
	// narrow and given a generous retry budget rather than failing the listing.
	detailFetchConcurrency = 3
	detailFetchMaxAttempts = 10
)

// ErrNotFound is returned when the named WAF object does not exist in the scope
// and region asked (WAFNonexistentItemException).
var ErrNotFound = stderrors.New("wafv2: resource not found")

type AwsClient interface {
	GetRegion() ptypes.AwsRegion
	GetAccountID() ptypes.AwsAccountID
}

type Wafv2Repository struct {
	ctx    context.Context
	client *v3.Client
}

func NewWafv2Repository(ctx context.Context, client *v3.Client) *Wafv2Repository {
	return &Wafv2Repository{
		ctx:    ctx,
		client: client,
	}
}

func (r *Wafv2Repository) GetRegion() ptypes.AwsRegion {
	return r.client.GetRegion()
}

func (r *Wafv2Repository) GetAccountID() ptypes.AwsAccountID {
	return r.client.GetAccountID()
}

func (r *Wafv2Repository) wafClient() *awswaf.Client {
	return wafclient.GetClient(r.client)
}

// Scopes returns the scopes a listing from this client covers: REGIONAL always,
// CLOUDFRONT only from us-east-1. See the package documentation.
func (r *Wafv2Repository) Scopes() []waftypes.Scope {
	return scopesForRegion(r.client.GetRegion())
}

func scopesForRegion(region ptypes.AwsRegion) []waftypes.Scope {
	if region == CloudFrontRegion {
		return []waftypes.Scope{waftypes.ScopeRegional, waftypes.ScopeCloudfront}
	}

	return []waftypes.Scope{waftypes.ScopeRegional}
}

// opts returns the per-operation options for a scope: CLOUDFRONT calls are
// pinned to us-east-1. The service client itself is shared and cached per v3
// client, so the region has to be overridden per call rather than at
// construction.
func (r *Wafv2Repository) opts(scope waftypes.Scope, extra ...func(*awswaf.Options)) []func(*awswaf.Options) {
	out := make([]func(*awswaf.Options), 0, len(extra)+1)

	if scope == waftypes.ScopeCloudfront && r.client.GetRegion() != CloudFrontRegion {
		out = append(out, func(o *awswaf.Options) { o.Region = CloudFrontRegion.String() })
	}

	return append(out, extra...)
}

func withRetries(o *awswaf.Options) {
	o.RetryMaxAttempts = detailFetchMaxAttempts
}

func (r *Wafv2Repository) promLabels(method string, resourceType cfg.ResourceType) prometheus.Labels {
	return prometheus.Labels{
		"account_id":    r.client.GetAccountID().String(),
		"region":        r.client.GetRegion().String(),
		"resource_type": ccfg.ResourceTypeToString(resourceType),
		"method":        method,
	}
}

func (r *Wafv2Repository) countRequest(method string, resourceType cfg.ResourceType) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels(method, resourceType)).Inc()
	}
}

func (r *Wafv2Repository) countError(method string, resourceType cfg.ResourceType) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequestErrors.With(r.promLabels(method, resourceType)).Inc()
	}
}

func (r *Wafv2Repository) countFetched(method string, resourceType cfg.ResourceType, n int) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.With(r.promLabels(method, resourceType)).Add(float64(n))
	}
}

// classifyErr maps WAFNonexistentItemException onto ErrNotFound, keeping the
// original error, and wraps everything with a stack trace.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}

	var missing *waftypes.WAFNonexistentItemException
	if stderrors.As(err, &missing) {
		return errors.New(fmt.Errorf("%w: %w", ErrNotFound, err))
	}

	return errors.New(err)
}

// ParseScope accepts "REGIONAL"/"CLOUDFRONT" in any case, plus "global" for
// CLOUDFRONT, which is how the ARN spells it.
func ParseScope(raw string) (waftypes.Scope, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "REGIONAL":
		return waftypes.ScopeRegional, nil
	case "CLOUDFRONT", "GLOBAL":
		return waftypes.ScopeCloudfront, nil
	default:
		return "", errors.Errorf("unknown WAF scope %q: expected REGIONAL or CLOUDFRONT", raw)
	}
}

// ObjectRef identifies a WAFv2 object by the parts of its ARN:
// arn:<partition>:wafv2:<region>:<account>:<regional|global>/<kind>/<name>/<id>.
type ObjectRef struct {
	ARN       string
	AccountID ptypes.AwsAccountID
	Region    ptypes.AwsRegion
	Scope     waftypes.Scope
	// Kind is the resource segment: webacl, rulegroup, ipset, regexpatternset or
	// managedruleset.
	Kind string
	Name string
	Id   string
}

// ParseArn splits a WAFv2 ARN into its scope, kind, name and id.
func ParseArn(raw string) (ObjectRef, error) {
	parsed, err := arn.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ObjectRef{}, errors.Errorf("invalid WAFv2 ARN %q: %w", raw, err)
	}

	if parsed.Service != "wafv2" {
		return ObjectRef{}, errors.Errorf("not a WAFv2 ARN (service %q): %s", parsed.Service, raw)
	}

	parts := strings.Split(parsed.Resource, "/")
	if len(parts) != 4 {
		return ObjectRef{}, errors.Errorf("unexpected WAFv2 ARN resource %q: want <scope>/<kind>/<name>/<id>", parsed.Resource)
	}

	scope, err := ParseScope(parts[0])
	if err != nil {
		return ObjectRef{}, err
	}

	return ObjectRef{
		ARN:       parsed.String(),
		AccountID: ptypes.AwsAccountID(parsed.AccountID),
		Region:    ptypes.AwsRegion(parsed.Region),
		Scope:     scope,
		Kind:      parts[1],
		Name:      parts[2],
		Id:        parts[3],
	}, nil
}

// parseArn turns an ARN returned by the API into the parsed form the resource
// abstraction expects. WAFv2 always returns complete ARNs.
func parseArn(value *string) *arn.ARN {
	if value == nil || *value == "" {
		return nil
	}

	parsed, err := arn.Parse(*value)
	if err != nil {
		return nil
	}

	return &parsed
}

// listTags reads every tag of one WAFv2 object.
func (r *Wafv2Repository) listTags(scope waftypes.Scope, resourceArn *string, resourceType cfg.ResourceType) ([]waftypes.Tag, error) {
	if aws.ToString(resourceArn) == "" {
		return nil, errors.New("resource ARN cannot be empty")
	}

	var tags []waftypes.Tag
	input := &awswaf.ListTagsForResourceInput{ResourceARN: resourceArn, Limit: aws.Int32(listPageSize)}

	for {
		r.countRequest("ListTagsForResource", resourceType)

		output, err := r.wafClient().ListTagsForResource(r.ctx, input, r.opts(scope, withRetries)...)
		if err != nil {
			r.countError("ListTagsForResource", resourceType)

			return nil, classifyErr(err)
		}

		if output.TagInfoForResource != nil {
			tags = append(tags, output.TagInfoForResource.TagList...)
		}

		if aws.ToString(output.NextMarker) == "" {
			return tags, nil
		}

		input.NextMarker = output.NextMarker
	}
}

// forEachBounded runs fn for every index with at most detailFetchConcurrency in
// flight, and returns the first error by index.
func forEachBounded(n int, fn func(i int) error) error {
	errs := make([]error, n)
	sem := make(chan struct{}, detailFetchConcurrency)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)

		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			errs[idx] = fn(idx)
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func tagMap(tags []waftypes.Tag) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		out[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return out
}
