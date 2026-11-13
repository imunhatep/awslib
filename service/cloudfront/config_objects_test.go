package cloudfront

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachePolicyEntity(t *testing.T) {
	policy := cftypes.CachePolicy{
		Id:                aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
		CachePolicyConfig: &cftypes.CachePolicyConfig{Name: aws.String("Managed-CachingOptimized"), Comment: aws.String("default")},
	}

	managed := NewCachePolicy(stubClient{}, policy, cftypes.CachePolicyTypeManaged)
	assert.Equal(t, "658327ea-f89d-4fab-a63d-7e88639e58f6", managed.GetId())
	assert.Equal(t, "Managed-CachingOptimized", managed.GetName())
	assert.Equal(t, ccfg.ResourceTypeCloudFrontCachePolicy, managed.GetType())
	assert.Equal(t, "managed", managed.GetPolicyType())
	assert.True(t, managed.IsManaged())
	assert.Empty(t, managed.GetArn(), "the API reports no ARN for a policy, and none is synthesized")
	assert.Empty(t, managed.GetTags())

	assert.False(t, NewCachePolicy(stubClient{}, policy, cftypes.CachePolicyTypeCustom).IsManaged())

	// AbstractResource.Type and a summary's Type would collide and both vanish from
	// JSON; carrying the policy type under its own name keeps it in the raw view.
	raw, err := json.Marshal(managed)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"PolicyType":"managed"`)
}

func TestOriginRequestAndResponseHeadersPolicies(t *testing.T) {
	orp := NewOriginRequestPolicy(stubClient{}, cftypes.OriginRequestPolicy{
		Id:                        aws.String("orp-1"),
		OriginRequestPolicyConfig: &cftypes.OriginRequestPolicyConfig{Name: aws.String("Managed-AllViewer")},
	}, cftypes.OriginRequestPolicyTypeManaged)
	assert.Equal(t, "Managed-AllViewer", orp.GetName())
	assert.True(t, orp.IsManaged())
	assert.Equal(t, ccfg.ResourceTypeCloudFrontOriginRequestPolicy, orp.GetType())

	rhp := NewResponseHeadersPolicy(stubClient{}, cftypes.ResponseHeadersPolicy{Id: aws.String("rhp-1")}, cftypes.ResponseHeadersPolicyTypeCustom)
	assert.Equal(t, "rhp-1", rhp.GetName(), "a policy without a config falls back to its ID")
	assert.Equal(t, "custom", rhp.GetPolicyType())
}

func TestFunctionEntity(t *testing.T) {
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	f := NewFunction(stubClient{}, cftypes.FunctionSummary{
		Name:   aws.String("rewrite-index"),
		Status: aws.String("UNASSOCIATED"),
		FunctionConfig: &cftypes.FunctionConfig{
			Runtime: cftypes.FunctionRuntimeCloudfrontJs20,
			KeyValueStoreAssociations: &cftypes.KeyValueStoreAssociations{Items: []cftypes.KeyValueStoreAssociation{
				{KeyValueStoreARN: aws.String("arn:aws:cloudfront::123456789012:key-value-store/kvs-1")},
			}},
		},
		FunctionMetadata: &cftypes.FunctionMetadata{
			FunctionARN: aws.String("arn:aws:cloudfront::123456789012:function/rewrite-index"),
			Stage:       cftypes.FunctionStageLive,
			CreatedTime: &created,
		},
	})

	assert.Equal(t, "rewrite-index", f.GetId())
	assert.Equal(t, "rewrite-index", f.GetName())
	assert.Equal(t, "arn:aws:cloudfront::123456789012:function/rewrite-index", f.GetArn())
	assert.Equal(t, created, f.GetCreatedAt())
	assert.Equal(t, "LIVE", f.GetStage())
	assert.Equal(t, "cloudfront-js-2.0", f.GetRuntime())
	assert.Equal(t, []string{"arn:aws:cloudfront::123456789012:key-value-store/kvs-1"}, f.GetKeyValueStoreArns())

	bare := NewFunction(stubClient{}, cftypes.FunctionSummary{Name: aws.String("x")})
	assert.Empty(t, bare.GetArn())
	assert.Empty(t, bare.GetStage())
	assert.Equal(t, epoch, bare.GetCreatedAt())
}

func TestSmallConfigEntities(t *testing.T) {
	oac := NewOriginAccessControl(stubClient{}, cftypes.OriginAccessControlSummary{
		Id: aws.String("E2QWRUHAPOMQZL"), Name: aws.String("assets-oac"),
		OriginAccessControlOriginType: cftypes.OriginAccessControlOriginTypesS3,
	})
	assert.Equal(t, "assets-oac", oac.GetName())
	assert.Equal(t, ccfg.ResourceTypeCloudFrontOriginAccessControl, oac.GetType())

	oai := NewOriginAccessIdentity(stubClient{}, cftypes.CloudFrontOriginAccessIdentitySummary{
		Id: aws.String("E1OAI"), Comment: aws.String("access-identity-assets"),
	})
	assert.Equal(t, "access-identity-assets", oai.GetName(), "an OAI's comment is its only label")
	assert.Equal(t, "E1OAI", NewOriginAccessIdentity(stubClient{}, cftypes.CloudFrontOriginAccessIdentitySummary{Id: aws.String("E1OAI")}).GetName())

	kg := NewKeyGroup(stubClient{}, cftypes.KeyGroup{
		Id:             aws.String("kg-1"),
		KeyGroupConfig: &cftypes.KeyGroupConfig{Name: aws.String("signers"), Items: []string{"K1", "K2"}},
	})
	assert.Equal(t, "signers", kg.GetName())
	ids := kg.GetPublicKeyIds()
	ids[0] = "mutated"
	assert.Equal(t, []string{"K1", "K2"}, kg.GetPublicKeyIds(), "GetPublicKeyIds must return a copy")

	rlc := NewRealtimeLogConfig(stubClient{}, cftypes.RealtimeLogConfig{
		Name: aws.String("rt-logs"),
		ARN:  aws.String("arn:aws:cloudfront::123456789012:realtime-log-config/rt-logs"),
		EndPoints: []cftypes.EndPoint{{
			StreamType:          aws.String("Kinesis"),
			KinesisStreamConfig: &cftypes.KinesisStreamConfig{StreamARN: aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/cf")},
		}},
	})
	assert.Equal(t, "rt-logs", rlc.GetId(), "a real-time log config has no ID other than its name")
	assert.Equal(t, "arn:aws:cloudfront::123456789012:realtime-log-config/rt-logs", rlc.GetArn())
	assert.Equal(t, []string{"arn:aws:kinesis:us-east-1:123456789012:stream/cf"}, rlc.GetStreamArns())

	cdp := NewContinuousDeploymentPolicy(stubClient{}, cftypes.ContinuousDeploymentPolicy{
		Id: aws.String("cdp-1"),
		ContinuousDeploymentPolicyConfig: &cftypes.ContinuousDeploymentPolicyConfig{
			Enabled:                     aws.Bool(true),
			StagingDistributionDnsNames: &cftypes.StagingDistributionDnsNames{Items: []string{"d9.cloudfront.net"}},
			TrafficConfig:               &cftypes.TrafficConfig{Type: cftypes.ContinuousDeploymentPolicyTypeSingleWeight},
		},
	})
	assert.Equal(t, "cdp-1", cdp.GetName())
	assert.True(t, cdp.IsEnabled())
	assert.Equal(t, "SingleWeight", cdp.GetTrafficType())
	assert.Equal(t, []string{"d9.cloudfront.net"}, cdp.GetStagingDomainNames())
}

func TestVpcOriginGetTagsReturnsCopy(t *testing.T) {
	vo := NewVpcOrigin(stubClient{}, cftypes.VpcOriginSummary{
		Id:                aws.String("vo_1"),
		Arn:               aws.String("arn:aws:cloudfront::123456789012:vpcorigin/vo_1"),
		Name:              aws.String("internal-alb"),
		OriginEndpointArn: aws.String("arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/internal/abc"),
	}, []cftypes.Tag{{Key: aws.String("service"), Value: aws.String("api")}})

	assert.Equal(t, "arn:aws:cloudfront::123456789012:vpcorigin/vo_1", vo.GetArn())
	vo.GetTags()["service"] = "mutated"
	assert.Equal(t, "api", vo.GetTagValue("service"))
}

func TestConfigEntitiesGobRoundTrip(t *testing.T) {
	original := []service.ResourceInterface{
		NewCachePolicy(stubClient{}, cftypes.CachePolicy{Id: aws.String("cp-1")}, cftypes.CachePolicyTypeCustom),
		NewFunction(stubClient{}, cftypes.FunctionSummary{Name: aws.String("f"), FunctionMetadata: &cftypes.FunctionMetadata{Stage: cftypes.FunctionStageDevelopment}}),
		NewVpcOrigin(stubClient{}, cftypes.VpcOriginSummary{Id: aws.String("vo_1")}, []cftypes.Tag{{Key: aws.String("k"), Value: aws.String("v")}}),
		NewKeyValueStore(stubClient{}, cftypes.KeyValueStore{Id: aws.String("kvs-1"), Name: aws.String("flags"), Status: aws.String("READY")}),
	}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(original))

	var decoded []service.ResourceInterface
	require.NoError(t, gob.NewDecoder(&buf).Decode(&decoded))
	require.Len(t, decoded, 4)

	assert.Equal(t, "custom", decoded[0].(CachePolicy).GetPolicyType())
	assert.Equal(t, "DEVELOPMENT", decoded[1].(Function).GetStage())
	assert.Equal(t, "v", decoded[2].(VpcOrigin).GetTagValue("k"))
	assert.Equal(t, "READY", decoded[3].(KeyValueStore).GetStatus())
}

// configStub serves the CloudFront list calls these tests exercise, plus STS and
// the tagging sweep.
type configStub struct {
	mu            sync.Mutex
	functionStage []string // Stage query parameter of every ListFunctions call
	cacheMarkers  []string // Marker query parameter of every ListCachePolicies call
	tagged        []string // ARNs passed to ListTagsForResource
}

const cfNS = `xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"`

func stubConfigObjects(t *testing.T, cachePages map[string]string) (*v3.Client, *configStub) {
	t.Helper()

	stub := &configStub{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.mu.Lock()
		defer stub.mu.Unlock()

		switch {
		case strings.HasSuffix(r.Header.Get("X-Amz-Target"), ".GetResources"):
			// The sweep reports a distribution but no VPC origin: whether VPC origins
			// are indexed is unknown, so their absence must not read as "untagged".
			w.Header().Set("Content-Type", "application/x-amz-json-1.1")
			_, _ = w.Write([]byte(`{"ResourceTagMappingList":[{"ResourceARN":"arn:aws:cloudfront::123456789012:distribution/E1","Tags":[]}]}`))
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(stsIdentity))
		case strings.HasSuffix(r.URL.Path, "/function"):
			stage := r.URL.Query().Get("Stage")
			stub.functionStage = append(stub.functionStage, stage)
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(`<FunctionList ` + cfNS + `><MaxItems>100</MaxItems><Quantity>1</Quantity><Items><FunctionSummary>` +
				`<Name>f1</Name><Status>DEPLOYED</Status><FunctionConfig><Comment>c</Comment><Runtime>cloudfront-js-2.0</Runtime></FunctionConfig>` +
				`<FunctionMetadata><FunctionARN>arn:aws:cloudfront::123456789012:function/f1</FunctionARN><Stage>` + stage + `</Stage>` +
				`<LastModifiedTime>2024-01-01T00:00:00Z</LastModifiedTime></FunctionMetadata></FunctionSummary></Items></FunctionList>`))
		case strings.HasSuffix(r.URL.Path, "/cache-policy"):
			marker := r.URL.Query().Get("Marker")
			stub.cacheMarkers = append(stub.cacheMarkers, marker)
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(cachePages[marker]))
		case strings.HasSuffix(r.URL.Path, "/vpc-origin"):
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(`<VpcOriginList ` + cfNS + `><Marker></Marker><MaxItems>100</MaxItems><IsTruncated>false</IsTruncated><Quantity>1</Quantity><Items><VpcOriginSummary>` +
				`<Id>vo_1</Id><Name>internal-alb</Name><Arn>arn:aws:cloudfront::123456789012:vpcorigin/vo_1</Arn><Status>Deployed</Status>` +
				`<OriginEndpointArn>arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/internal/abc</OriginEndpointArn>` +
				`<CreatedTime>2024-01-01T00:00:00Z</CreatedTime><LastModifiedTime>2024-01-01T00:00:00Z</LastModifiedTime>` +
				`</VpcOriginSummary></Items></VpcOriginList>`))
		case strings.HasSuffix(r.URL.Path, "/tagging"):
			stub.tagged = append(stub.tagged, r.URL.Query().Get("Resource"))
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(`<Tags><Items><Tag><Key>service</Key><Value>api</Value></Tag></Items></Tags>`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client, err := v3.NewClient(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithBaseEndpoint(srv.URL),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithRetryMaxAttempts(1),
	)
	require.NoError(t, err)

	return client, stub
}

func cachePolicyPage(id, policyType, next string) string {
	nextMarker := ""
	if next != "" {
		nextMarker = `<NextMarker>` + next + `</NextMarker>`
	}

	return `<CachePolicyList ` + cfNS + `>` + nextMarker + `<MaxItems>1</MaxItems><Quantity>1</Quantity><Items><CachePolicySummary>` +
		`<Type>` + policyType + `</Type><CachePolicy><Id>` + id + `</Id><LastModifiedTime>2024-01-01T00:00:00Z</LastModifiedTime>` +
		`<CachePolicyConfig><Name>` + id + `-name</Name><MinTTL>0</MinTTL></CachePolicyConfig></CachePolicy></CachePolicySummary></Items></CachePolicyList>`
}

// A request without a Type filter returns managed and custom policies alike, and
// the listing must follow NextMarker rather than stop at the first page.
func TestListCachePoliciesFollowsMarker(t *testing.T) {
	client, stub := stubConfigObjects(t, map[string]string{
		"":      cachePolicyPage("managed-1", "managed", "page2"),
		"page2": cachePolicyPage("custom-1", "custom", ""),
	})

	items, err := NewCloudFrontRepository(context.Background(), client).ListCachePoliciesAll()
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Equal(t, []string{"", "page2"}, stub.cacheMarkers)
	assert.Equal(t, "managed-1-name", items[0].GetName())
	assert.True(t, items[0].IsManaged())
	assert.Equal(t, "custom", items[1].GetPolicyType())
}

func TestListCachePoliciesRejectsRepeatedMarker(t *testing.T) {
	client, _ := stubConfigObjects(t, map[string]string{
		"":     cachePolicyPage("a", "custom", "loop"),
		"loop": cachePolicyPage("b", "custom", "loop"),
	})

	_, err := NewCloudFrontRepository(context.Background(), client).ListCachePoliciesAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned twice")
}

// ListFunctions filters on one stage per call; both have to be asked for, or a
// never-published function — or the code actually serving traffic — goes missing.
func TestListFunctionsAllListsBothStages(t *testing.T) {
	client, stub := stubConfigObjects(t, nil)

	items, err := NewCloudFrontRepository(context.Background(), client).ListFunctionsAll()
	require.NoError(t, err)
	require.Len(t, items, 2)

	assert.Equal(t, []string{"LIVE", "DEVELOPMENT"}, stub.functionStage)
	assert.Equal(t, "LIVE", items[0].GetStage())
	assert.Equal(t, "DEVELOPMENT", items[1].GetStage())
	assert.Equal(t, items[0].GetArn(), items[1].GetArn(), "both stages share the function's ARN")
}

// VPC origins have not been observed in a tagging sweep, so a sweep that reports
// none must not be read as "every VPC origin is untagged".
func TestListVpcOriginsReadsTagsPerOriginWhenSweepCannotVouch(t *testing.T) {
	client, stub := stubConfigObjects(t, nil)

	items, err := NewCloudFrontRepository(context.Background(), client).ListVpcOriginsAll()
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, []string{"arn:aws:cloudfront::123456789012:vpcorigin/vo_1"}, stub.tagged)
	assert.Equal(t, "api", items[0].GetTagValue("service"))
	assert.Equal(t, "internal-alb", items[0].GetName())
	assert.Equal(t, "arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/internal/abc", items[0].GetOriginEndpointArn())
}
