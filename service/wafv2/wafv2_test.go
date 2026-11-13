package wafv2

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const stsIdentity = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123456789012:user/test</Arn>
    <UserId>AIDATEST</UserId>
    <Account>123456789012</Account>
  </GetCallerIdentityResult>
</GetCallerIdentityResponse>`

type fakeClient struct{ region ptypes.AwsRegion }

func (f fakeClient) GetRegion() ptypes.AwsRegion       { return f.region }
func (f fakeClient) GetAccountID() ptypes.AwsAccountID { return "123456789012" }

const regionalACLArn = "arn:aws:wafv2:eu-central-1:123456789012:regional/webacl/edge/a1"

func sampleWebACL() waftypes.WebACL {
	return waftypes.WebACL{
		ARN:           aws.String(regionalACLArn),
		Id:            aws.String("a1"),
		Name:          aws.String("edge"),
		Capacity:      700,
		DefaultAction: &waftypes.DefaultAction{Allow: &waftypes.AllowAction{}},
		VisibilityConfig: &waftypes.VisibilityConfig{
			CloudWatchMetricsEnabled: true,
			MetricName:               aws.String("edge-metric"),
			SampledRequestsEnabled:   true,
		},
		Rules: []waftypes.Rule{
			{
				Name:           aws.String("aws-common"),
				Priority:       1,
				OverrideAction: &waftypes.OverrideAction{None: &waftypes.NoneAction{}},
				Statement: &waftypes.Statement{ManagedRuleGroupStatement: &waftypes.ManagedRuleGroupStatement{
					VendorName: aws.String("AWS"),
					Name:       aws.String("AWSManagedRulesCommonRuleSet"),
				}},
				VisibilityConfig: &waftypes.VisibilityConfig{MetricName: aws.String("aws-common")},
			},
			{
				Name:     aws.String("query-block"),
				Priority: 2,
				Action:   &waftypes.RuleAction{Block: &waftypes.BlockAction{}},
				Statement: &waftypes.Statement{ByteMatchStatement: &waftypes.ByteMatchStatement{
					SearchString:         []byte("select"),
					FieldToMatch:         &waftypes.FieldToMatch{AllQueryArguments: &waftypes.AllQueryArguments{}},
					PositionalConstraint: waftypes.PositionalConstraintContains,
				}},
				VisibilityConfig: &waftypes.VisibilityConfig{MetricName: aws.String("query-block")},
			},
		},
	}
}

// TestEntitiesRoundTripThroughGob pins the JSON-backed gob encoding (gob.go):
// the raw SDK types fail with "type has no exported fields", which would make
// every cached WAF listing a cache miss — or, through the generated wrapper, an
// error.
func TestEntitiesRoundTripThroughGob(t *testing.T) {
	client := fakeClient{region: "eu-central-1"}

	acl := NewWebACL(client, waftypes.ScopeRegional, sampleWebACL(), []waftypes.Tag{{Key: aws.String("team"), Value: aws.String("edge")}})
	acl.LoggingKnown = true
	acl.Logging = &waftypes.LoggingConfiguration{
		ResourceArn:           aws.String(regionalACLArn),
		LogDestinationConfigs: []string{"arn:aws:logs:eu-central-1:123456789012:log-group:aws-waf-logs-edge"},
		RedactedFields:        []waftypes.FieldToMatch{{AllQueryArguments: &waftypes.AllQueryArguments{}}},
	}

	group := NewRuleGroup(client, waftypes.ScopeRegional, waftypes.RuleGroup{
		ARN:   aws.String("arn:aws:wafv2:eu-central-1:123456789012:regional/rulegroup/rg/r1"),
		Id:    aws.String("r1"),
		Name:  aws.String("rg"),
		Rules: sampleWebACL().Rules,
	}, nil)

	ipSet := NewIPSet(client, waftypes.ScopeRegional, waftypes.IPSet{
		ARN: aws.String("arn:aws:wafv2:eu-central-1:123456789012:regional/ipset/office/i1"), Id: aws.String("i1"), Name: aws.String("office"),
		Addresses: []string{"10.0.0.0/8"}, IPAddressVersion: waftypes.IPAddressVersionIpv4,
	}, nil)

	regexSet := NewRegexPatternSet(client, waftypes.ScopeRegional, waftypes.RegexPatternSet{
		ARN: aws.String("arn:aws:wafv2:eu-central-1:123456789012:regional/regexpatternset/bots/x1"), Id: aws.String("x1"), Name: aws.String("bots"),
		RegularExpressionList: []waftypes.Regex{{RegexString: aws.String("(?i)curl")}},
	}, nil)

	roundTrip := func(in, out any) {
		t.Helper()

		var buf bytes.Buffer
		require.NoError(t, gob.NewEncoder(&buf).Encode(in))
		require.NoError(t, gob.NewDecoder(&buf).Decode(out))
	}

	var acls []WebACL
	roundTrip([]WebACL{acl}, &acls)
	require.Len(t, acls, 1)
	assert.Equal(t, acl, acls[0])
	assert.Equal(t, regionalACLArn, acls[0].GetArn(), "ARN must survive although both embedded structs declare it")
	assert.NotNil(t, acls[0].Rules[0].OverrideAction.None, "an empty action must come back non-nil")
	assert.Equal(t, "allow", acls[0].GetDefaultAction())

	var groups []RuleGroup
	roundTrip([]RuleGroup{group}, &groups)
	assert.Equal(t, group, groups[0])

	var ipSets []IPSet
	roundTrip([]IPSet{ipSet}, &ipSets)
	assert.Equal(t, ipSet, ipSets[0])

	var regexSets []RegexPatternSet
	roundTrip([]RegexPatternSet{regexSet}, &regexSets)
	assert.Equal(t, regexSet, regexSets[0])

	// The cache may also hold entities behind the interface.
	var resources []service.ResourceInterface
	roundTrip([]service.ResourceInterface{acl, group}, &resources)
	assert.Equal(t, acl, resources[0])
}

func TestScopesAndRegions(t *testing.T) {
	assert.Equal(t, []waftypes.Scope{waftypes.ScopeRegional, waftypes.ScopeCloudfront}, scopesForRegion("us-east-1"))
	assert.Equal(t, []waftypes.Scope{waftypes.ScopeRegional}, scopesForRegion("eu-west-1"))

	client := fakeClient{region: "eu-west-1"}
	assert.Equal(t, ptypes.AwsRegion("us-east-1"), NewWebACL(client, waftypes.ScopeCloudfront, waftypes.WebACL{}, nil).GetRegion())
	assert.Equal(t, ptypes.AwsRegion("eu-west-1"), NewWebACL(client, waftypes.ScopeRegional, waftypes.WebACL{}, nil).GetRegion())
}

func TestParseArn(t *testing.T) {
	ref, err := ParseArn(regionalACLArn)
	require.NoError(t, err)
	assert.Equal(t, ObjectRef{ARN: regionalACLArn, AccountID: "123456789012", Region: "eu-central-1", Scope: waftypes.ScopeRegional, Kind: "webacl", Name: "edge", Id: "a1"}, ref)

	ref, err = ParseArn("arn:aws:wafv2:us-east-1:123456789012:global/webacl/cdn/b2")
	require.NoError(t, err)
	assert.Equal(t, waftypes.ScopeCloudfront, ref.Scope)
	assert.Equal(t, "cdn", ref.Name)

	for _, bad := range []string{"", "edge", "arn:aws:s3:::bucket", "arn:aws:wafv2:us-east-1:1:global/webacl/cdn", "arn:aws:wafv2:us-east-1:1:local/webacl/cdn/b2"} {
		_, err := ParseArn(bad)
		assert.Errorf(t, err, "%q must not parse", bad)
	}

	for raw, want := range map[string]waftypes.Scope{"regional": waftypes.ScopeRegional, "CloudFront": waftypes.ScopeCloudfront, "global": waftypes.ScopeCloudfront} {
		scope, err := ParseScope(raw)
		require.NoError(t, err)
		assert.Equal(t, want, scope)
	}
}

// wafCall is one request the stub received: the operation, the region the
// request was signed for, and its body.
type wafCall struct {
	op     string
	region string
	body   map[string]any
}

// stubWAF serves STS (so v3.NewClient resolves the account) and dispatches
// WAFv2 JSON calls by X-Amz-Target to handle.
func stubWAF(t *testing.T, region string, handle func(call wafCall) (int, any)) (*v3.Client, func() []wafCall) {
	t.Helper()

	var (
		mu    sync.Mutex
		calls []wafCall
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)

		target := r.Header.Get("X-Amz-Target")
		if target == "" {
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(stsIdentity))

			return
		}

		call := wafCall{op: strings.TrimPrefix(target, "AWSWAF_20190729."), region: signedRegion(r.Header.Get("Authorization"))}
		require.NoError(t, json.Unmarshal(raw, &call.body))

		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()

		status, resp := handle(call)
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.WriteHeader(status)
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)

	client, err := v3.NewClient(context.Background(),
		config.WithRegion(region),
		config.WithBaseEndpoint(srv.URL),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithRetryMaxAttempts(1),
	)
	require.NoError(t, err)

	return client, func() []wafCall {
		mu.Lock()
		defer mu.Unlock()

		return append([]wafCall(nil), calls...)
	}
}

// signedRegion extracts the region from a SigV4 credential scope.
func signedRegion(authorization string) string {
	_, scope, ok := strings.Cut(authorization, "Credential=")
	if !ok {
		return ""
	}

	parts := strings.Split(scope, "/")
	if len(parts) < 3 {
		return ""
	}

	return parts[2]
}

func wafError(kind string) (int, any) {
	return http.StatusBadRequest, map[string]any{"__type": kind, "Message": kind}
}

func TestListWebACLsAllReadsBothScopesFromUsEast1(t *testing.T) {
	client, calls := stubWAF(t, "us-east-1", func(call wafCall) (int, any) {
		scope, _ := call.body["Scope"].(string)

		switch call.op {
		case "ListWebACLs":
			if scope == "CLOUDFRONT" {
				return http.StatusOK, map[string]any{"WebACLs": []any{map[string]any{"Name": "cdn", "Id": "c1"}}}
			}

			return http.StatusOK, map[string]any{"WebACLs": []any{
				map[string]any{"Name": "edge", "Id": "a1"},
				map[string]any{"Name": "gone", "Id": "g1"},
			}}
		case "GetWebACL":
			name := call.body["Name"].(string)
			if name == "gone" {
				return wafError("WAFNonexistentItemException")
			}

			prefix := "regional"
			if scope == "CLOUDFRONT" {
				prefix = "global"
			}

			return http.StatusOK, map[string]any{"WebACL": map[string]any{
				"Name": name, "Id": call.body["Id"],
				"ARN":           "arn:aws:wafv2:us-east-1:123456789012:" + prefix + "/webacl/" + name + "/x",
				"DefaultAction": map[string]any{"Block": map[string]any{}},
				"VisibilityConfig": map[string]any{
					"CloudWatchMetricsEnabled": true, "MetricName": name + "-metric", "SampledRequestsEnabled": true,
				},
			}}
		case "ListTagsForResource":
			return http.StatusOK, map[string]any{"TagInfoForResource": map[string]any{
				"ResourceARN": call.body["ResourceARN"],
				"TagList":     []any{map[string]any{"Key": "team", "Value": "edge"}},
			}}
		case "ListLoggingConfigurations":
			if scope == "CLOUDFRONT" {
				return wafError("AccessDeniedException")
			}

			return http.StatusOK, map[string]any{"LoggingConfigurations": []any{map[string]any{
				"ResourceArn":           "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/edge/x",
				"LogDestinationConfigs": []any{"arn:aws:s3:::aws-waf-logs-edge"},
			}}}
		}

		t.Errorf("unexpected call %s", call.op)

		return http.StatusBadRequest, nil
	})

	acls, err := NewWafv2Repository(context.Background(), client).ListWebACLsAll()
	require.NoError(t, err)
	require.Len(t, acls, 2, "the ACL deleted between list and get is skipped, not an error")

	byName := map[string]WebACL{}
	for _, acl := range acls {
		byName[acl.GetName()] = acl
	}

	edge := byName["edge"]
	assert.Equal(t, waftypes.ScopeRegional, edge.Scope)
	assert.Equal(t, "block", edge.GetDefaultAction())
	assert.Equal(t, "edge-metric", edge.GetMetricName())
	assert.Equal(t, map[string]string{"team": "edge"}, edge.GetTags())
	assert.True(t, edge.LoggingKnown)
	assert.Equal(t, []string{"arn:aws:s3:::aws-waf-logs-edge"}, edge.GetLogDestinations())

	cdn := byName["cdn"]
	assert.Equal(t, waftypes.ScopeCloudfront, cdn.Scope)
	assert.Equal(t, ptypes.AwsRegion("us-east-1"), cdn.GetRegion())
	assert.False(t, cdn.LoggingKnown, "an unreadable logging sweep must not read as logging off")

	ops := map[string]int{}
	for _, call := range calls() {
		ops[call.op]++
	}

	assert.Equal(t, 2, ops["ListWebACLs"])
	assert.Equal(t, 3, ops["GetWebACL"])
	assert.Equal(t, 2, ops["ListLoggingConfigurations"], "logging is one sweep per scope, not a Get per ACL")
}

func TestCloudFrontScopeIsPinnedToUsEast1(t *testing.T) {
	client, calls := stubWAF(t, "eu-central-1", func(call wafCall) (int, any) {
		return http.StatusOK, map[string]any{"WebACLs": []any{}}
	})

	repo := NewWafv2Repository(context.Background(), client)

	_, err := repo.ListWebACLSummariesByScope(waftypes.ScopeCloudfront)
	require.NoError(t, err)
	_, err = repo.ListWebACLSummariesByScope(waftypes.ScopeRegional)
	require.NoError(t, err)

	got := calls()
	require.Len(t, got, 2)
	assert.Equal(t, "us-east-1", got[0].region)
	assert.Equal(t, "eu-central-1", got[1].region)

	// A listing from a non-us-east-1 client does not ask for CLOUDFRONT at all.
	assert.Equal(t, []waftypes.Scope{waftypes.ScopeRegional}, repo.Scopes())
}

func TestListingFailsWhenTagsAreUnreadable(t *testing.T) {
	client, _ := stubWAF(t, "eu-central-1", func(call wafCall) (int, any) {
		switch call.op {
		case "ListIPSets":
			return http.StatusOK, map[string]any{"IPSets": []any{map[string]any{"Name": "office", "Id": "i1"}}}
		case "GetIPSet":
			return http.StatusOK, map[string]any{"IPSet": map[string]any{
				"Name": "office", "Id": "i1", "ARN": "arn:aws:wafv2:eu-central-1:123456789012:regional/ipset/office/i1",
				"Addresses": []any{"10.0.0.0/8"}, "IPAddressVersion": "IPV4",
			}}
		case "ListTagsForResource":
			return wafError("WAFInternalErrorException")
		}

		return http.StatusBadRequest, nil
	})

	_, err := NewWafv2Repository(context.Background(), client).ListIPSetsAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading tags of IP set office")
}

func TestWebACLAssociationsReportPartialFailure(t *testing.T) {
	client, _ := stubWAF(t, "eu-central-1", func(call wafCall) (int, any) {
		require.Equal(t, "ListResourcesForWebACL", call.op)

		switch call.body["ResourceType"] {
		case "APPLICATION_LOAD_BALANCER":
			return http.StatusOK, map[string]any{"ResourceArns": []any{"arn:aws:elasticloadbalancing:eu-central-1:123456789012:loadbalancer/app/web/1"}}
		case "VERIFIED_ACCESS_INSTANCE":
			return wafError("WAFInvalidParameterException")
		}

		return http.StatusOK, map[string]any{"ResourceArns": []any{}}
	})

	repo := NewWafv2Repository(context.Background(), client)
	acl := NewWebACL(fakeClient{region: "eu-central-1"}, waftypes.ScopeRegional, sampleWebACL(), nil)

	found, err := repo.FindWebACLAssociations(acl)
	require.NoError(t, err)
	require.Len(t, found.Resources, 1)
	assert.Equal(t, "APPLICATION_LOAD_BALANCER", found.Resources[0].ResourceType)
	assert.Len(t, found.Checked, len(regionalAssociationTypes))
	assert.Contains(t, found.Failures, "VERIFIED_ACCESS_INSTANCE")

	failing, _ := stubWAF(t, "eu-central-1", func(call wafCall) (int, any) { return wafError("AccessDeniedException") })

	_, err = NewWafv2Repository(context.Background(), failing).FindWebACLAssociations(acl)
	assert.Error(t, err, "no type checked must be an error, not an unprotected ACL")
}

func metric(dims ...string) cwtypes.Metric {
	m := cwtypes.Metric{Namespace: aws.String(MetricNamespace), MetricName: aws.String("BlockedRequests")}
	for i := 0; i+1 < len(dims); i += 2 {
		m.Dimensions = append(m.Dimensions, cwtypes.Dimension{Name: aws.String(dims[i]), Value: aws.String(dims[i+1])})
	}

	return m
}

func TestSelectRegion(t *testing.T) {
	found := []cwtypes.Metric{
		metric(DimWebACL, "edge", DimRegion, "eu-central-1", DimRule, "ALL"),
		metric(DimWebACL, "edge", DimRegion, "eu-west-1", DimRule, "ALL"),
		metric(DimWebACL, "edge", DimRule, "ALL"),
	}

	regional, warning := selectRegion(found, waftypes.ScopeRegional, "eu-central-1")
	assert.Equal(t, found[:1], regional)
	assert.Empty(t, warning)

	global, warning := selectRegion(found, waftypes.ScopeCloudfront, "us-east-1")
	assert.Equal(t, found[2:], global, "CloudFront metrics have no Region dimension")
	assert.Empty(t, warning)

	onlyRegional := []cwtypes.Metric{metric(DimWebACL, "cdn", DimRegion, "us-east-1", DimRule, "ALL")}
	used, warning := selectRegion(onlyRegional, waftypes.ScopeCloudfront, "us-east-1")
	assert.Equal(t, onlyRegional, used)
	assert.NotEmpty(t, warning, "falling back to Region=us-east-1 must be said")
}

func TestSelectDimensions(t *testing.T) {
	found := []cwtypes.Metric{
		metric(DimWebACL, "edge", DimRule, "ALL"),
		metric(DimWebACL, "edge", DimRuleGroup, "aws-common", DimRule, "SizeRestrictions_BODY"),
		metric(DimWebACL, "edge", DimLabelNamespace, "awswaf:managed", DimLabel, "x"),
		metric(DimWebACL, "edge", "Country", "DE", DimRule, "ALL"),
	}

	skipped := map[string]int{}
	assert.Len(t, selectDimensions(found, false, skipped), 2)
	assert.Equal(t, 1, skipped["Country"])
	assert.Equal(t, 1, skipped[DimLabel]+skipped[DimLabelNamespace])

	skipped = map[string]int{}
	assert.Len(t, selectDimensions(found, true, skipped), 3)
	assert.Equal(t, map[string]int{"Country": 1}, skipped)

	assert.Equal(t, map[string]string{DimRuleGroup: "aws-common", DimRule: "SizeRestrictions_BODY"}, ruleDims(found[1].Dimensions))
}

func TestFetchSampledRequestsValidatesBeforeCalling(t *testing.T) {
	repo := &Wafv2Repository{ctx: context.Background()}
	acl := NewWebACL(fakeClient{region: "eu-central-1"}, waftypes.ScopeRegional, sampleWebACL(), nil)
	now := time.Now()

	for _, query := range []SampledRequestsQuery{
		{RuleMetricName: "", Start: now.Add(-time.Hour), End: now, MaxItems: 10},
		{RuleMetricName: "x", Start: now.Add(-time.Hour), End: now, MaxItems: 0},
		{RuleMetricName: "x", Start: now.Add(-time.Hour), End: now, MaxItems: 501},
		{RuleMetricName: "x", Start: now, End: now.Add(-time.Hour), MaxItems: 10},
	} {
		// repo has no client: reaching AWS would panic.
		_, err := repo.FetchSampledRequests(acl, query)
		assert.Error(t, err)
	}
}
