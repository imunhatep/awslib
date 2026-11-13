package wafv2

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awswaf "github.com/aws/aws-sdk-go-v2/service/wafv2"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/go-errors/errors"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

const (
	// SampledRequestsMaxWindow is how far back GetSampledRequests can look: the
	// window must start within the previous three hours.
	SampledRequestsMaxWindow = 3 * time.Hour
	// SampledRequestsMaxItems is the most requests one call returns.
	SampledRequestsMaxItems = 500
)

// SampledRequestsQuery selects a rule and a window for FetchSampledRequests.
type SampledRequestsQuery struct {
	// RuleMetricName is the metric name of the rule to sample — a top-level
	// rule's VisibilityConfig.MetricName, a rule group reference's metric name
	// for requests matched inside the group, or Default_Action.
	RuleMetricName string
	Start          time.Time
	End            time.Time
	// MaxItems is 1..SampledRequestsMaxItems.
	MaxItems int64
}

// SampledRequests is a sample of the web requests a rule matched.
type SampledRequests struct {
	// PopulationSize is how many requests the rule matched in the window, of
	// which Requests is a sample; each request's Weight says how many it stands
	// for.
	PopulationSize int64
	Start          time.Time
	End            time.Time
	Requests       []waftypes.SampledHTTPRequest
}

// FetchSampledRequests returns up to MaxItems of the requests a rule matched in
// a window within the last three hours. Sampling must be enabled on the rule
// (VisibilityConfig.SampledRequestsEnabled); otherwise the sample is empty.
//
// Deliberately not a Get*/List* method: a sample of live traffic must never be
// answered from a cache.
func (r *Wafv2Repository) FetchSampledRequests(acl WebACL, query SampledRequestsQuery) (*SampledRequests, error) {
	if query.RuleMetricName == "" {
		return nil, errors.New("rule metric name is required")
	}

	if query.MaxItems < 1 || query.MaxItems > SampledRequestsMaxItems {
		return nil, errors.Errorf("max items must be 1..%d, got %d", SampledRequestsMaxItems, query.MaxItems)
	}

	if !query.End.After(query.Start) {
		return nil, errors.New("end must be after start")
	}

	r.countRequest("GetSampledRequests", ccfg.ResourceTypeWafv2SampledRequest)

	output, err := r.wafClient().GetSampledRequests(r.ctx, &awswaf.GetSampledRequestsInput{
		WebAclArn:      acl.WebACL.ARN,
		RuleMetricName: aws.String(query.RuleMetricName),
		Scope:          acl.Scope,
		MaxItems:       aws.Int64(query.MaxItems),
		TimeWindow: &waftypes.TimeWindow{
			StartTime: aws.Time(query.Start.UTC()),
			EndTime:   aws.Time(query.End.UTC()),
		},
	}, r.opts(acl.Scope, withRetries)...)
	if err != nil {
		r.countError("GetSampledRequests", ccfg.ResourceTypeWafv2SampledRequest)

		return nil, classifyErr(err)
	}

	out := &SampledRequests{
		PopulationSize: output.PopulationSize,
		Start:          query.Start.UTC(),
		End:            query.End.UTC(),
		Requests:       output.SampledRequests,
	}

	// WAF reports the window it actually sampled, which can be narrower.
	if output.TimeWindow != nil {
		out.Start = aws.ToTime(output.TimeWindow.StartTime)
		out.End = aws.ToTime(output.TimeWindow.EndTime)
	}

	return out, nil
}
