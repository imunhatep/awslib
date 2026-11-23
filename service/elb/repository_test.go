package elb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const elbNS = "http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/"

// elbStub answers the ELBv2 Query API from per-Action handlers and records the
// form of every call.
type elbStub struct {
	mu    sync.Mutex
	calls map[string][]map[string][]string
}

func (s *elbStub) count(action string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.calls[action])
}

func newELBStub(t *testing.T, routes map[string]func(w http.ResponseWriter, form map[string][]string)) (*LoadBalancerRepository, *elbStub) {
	t.Helper()

	stub := &elbStub{calls: map[string][]map[string][]string{}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		action := r.PostForm.Get("Action")

		stub.mu.Lock()
		stub.calls[action] = append(stub.calls[action], r.PostForm)
		stub.mu.Unlock()

		route, ok := routes[action]
		if !ok {
			t.Errorf("unexpected action %s", action)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		route(w, r.PostForm)
	}))
	t.Cleanup(srv.Close)

	client, err := v3.NewClientForAccount(context.Background(), "123456789012",
		config.WithRegion("eu-west-1"),
		config.WithBaseEndpoint(srv.URL),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKIATEST", "secret", "")),
		config.WithRetryMaxAttempts(1),
	)
	require.NoError(t, err)

	return NewLoadBalancerRepository(context.Background(), client), stub
}

func respond(w http.ResponseWriter, action, result string) {
	w.Header().Set("Content-Type", "text/xml")
	_, _ = fmt.Fprintf(w, `<%sResponse xmlns="%s"><%sResult>%s</%sResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></%sResponse>`,
		action, elbNS, action, result, action, action)
}

func fail(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = fmt.Fprintf(w, `<ErrorResponse xmlns="%s"><Error><Type>Sender</Type><Code>%s</Code><Message>no</Message></Error><RequestId>r</RequestId></ErrorResponse>`, elbNS, code)
}

func tgArnN(i int) string {
	return fmt.Sprintf("arn:aws:elasticloadbalancing:eu-west-1:123456789012:targetgroup/tg-%02d/%016x", i, i)
}

// tagsFor answers DescribeTags with service=<ARN> for every requested ARN.
func tagsFor(w http.ResponseWriter, form map[string][]string) {
	var b strings.Builder
	for i := 1; ; i++ {
		v, ok := form[fmt.Sprintf("ResourceArns.member.%d", i)]
		if !ok {
			break
		}
		fmt.Fprintf(&b, `<member><ResourceArn>%s</ResourceArn><Tags><member><Key>service</Key><Value>%s</Value></member></Tags></member>`, v[0], v[0])
	}

	respond(w, "DescribeTags", "<TagDescriptions>"+b.String()+"</TagDescriptions>")
}

// targetGroupPages serves 25 target groups over two pages.
func targetGroupPages(w http.ResponseWriter, form map[string][]string) {
	from, to, next := 1, 15, "<NextMarker>page2</NextMarker>"
	if len(form["Marker"]) > 0 && form["Marker"][0] == "page2" {
		from, to, next = 16, 25, ""
	}

	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, `<member><TargetGroupArn>%s</TargetGroupArn><TargetGroupName>tg-%02d</TargetGroupName><LoadBalancerArns><member>%s</member></LoadBalancerArns></member>`, tgArnN(i), i, albArn)
	}

	respond(w, "DescribeTargetGroups", "<TargetGroups>"+b.String()+"</TargetGroups>"+next)
}

func TestListTargetGroupsAll(t *testing.T) {
	repo, stub := newELBStub(t, map[string]func(http.ResponseWriter, map[string][]string){
		"DescribeTargetGroups": targetGroupPages,
		"DescribeTags":         tagsFor,
		"DescribeTargetHealth": func(w http.ResponseWriter, form map[string][]string) {
			if form["TargetGroupArn"][0] == tgArnN(7) {
				fail(w, "AccessDenied")
				return
			}

			// Deliberately unsorted: the listing sorts by target ID.
			respond(w, "DescribeTargetHealth", `<TargetHealthDescriptions>
<member><Target><Id>i-2</Id><Port>80</Port></Target><TargetHealth><State>unhealthy</State><Reason>Target.Timeout</Reason></TargetHealth></member>
<member><Target><Id>i-1</Id><Port>80</Port></Target><TargetHealth><State>healthy</State></TargetHealth></member>
</TargetHealthDescriptions>`)
		},
	})

	groups, err := repo.ListTargetGroupsAll()
	require.NoError(t, err, "unreadable health for one target group does not fail the listing")
	require.Len(t, groups, 25)

	assert.Equal(t, 2, stub.count("DescribeTargetGroups"))
	assert.Equal(t, 2, stub.count("DescribeTags"), "25 ARNs at 20 per DescribeTags call")
	assert.Equal(t, 25, stub.count("DescribeTargetHealth"))

	for i, g := range groups {
		assert.Equal(t, fmt.Sprintf("tg-%02d", i+1), g.GetName(), "listing order is preserved")
		assert.Equal(t, g.GetArn(), g.GetTagValue("service"))

		if i+1 == 7 {
			assert.False(t, g.HealthKnown())
			assert.Contains(t, g.HealthError, "AccessDenied")
			assert.Nil(t, g.Targets)
			continue
		}

		assert.True(t, g.HealthKnown())
		assert.False(t, g.HealthCheckedAt.IsZero())
		assert.Equal(t, []string{"i-1", "i-2"}, g.TargetIDs())
		assert.Equal(t, 1, g.HealthyCount())
		assert.Equal(t, 1, g.UnhealthyCount())
		assert.Equal(t, types.TargetHealthReasonEnumTimeout, g.Targets[1].Reason)
	}
}

// Untagged and "tags unknown" look the same once returned, so unreadable tags fail the listing.
func TestListTargetGroupsFailsOnTags(t *testing.T) {
	repo, stub := newELBStub(t, map[string]func(http.ResponseWriter, map[string][]string){
		"DescribeTargetGroups": targetGroupPages,
		"DescribeTags":         func(w http.ResponseWriter, _ map[string][]string) { fail(w, "AccessDenied") },
	})

	groups, err := repo.ListTargetGroupsAll()
	require.Error(t, err)
	assert.Nil(t, groups)
	assert.Zero(t, stub.count("DescribeTargetHealth"))
}

func lbPage(arns ...string) func(http.ResponseWriter, map[string][]string) {
	return func(w http.ResponseWriter, _ map[string][]string) {
		var b strings.Builder
		for _, a := range arns {
			fmt.Fprintf(&b, `<member><LoadBalancerArn>%s</LoadBalancerArn></member>`, a)
		}
		respond(w, "DescribeLoadBalancers", "<LoadBalancers>"+b.String()+"</LoadBalancers>")
	}
}

func TestListListenersAll(t *testing.T) {
	repo, stub := newELBStub(t, map[string]func(http.ResponseWriter, map[string][]string){
		"DescribeLoadBalancers": lbPage(albArn, nlbArn),
		"DescribeTags":          tagsFor,
		"DescribeListeners": func(w http.ResponseWriter, form map[string][]string) {
			if form["LoadBalancerArn"][0] == nlbArn {
				fail(w, "LoadBalancerNotFound")
				return
			}

			respond(w, "DescribeListeners", `<Listeners>
<member><ListenerArn>`+albListenerArn+`</ListenerArn><LoadBalancerArn>`+albArn+`</LoadBalancerArn><Protocol>HTTPS</Protocol><Port>443</Port>
<DefaultActions><member><Type>forward</Type><TargetGroupArn>`+tgArn+`</TargetGroupArn></member></DefaultActions></member>
</Listeners>`)
		},
	})

	listeners, err := repo.ListListenersAll()
	require.NoError(t, err, "a load balancer deleted mid-listing has no listeners")
	require.Len(t, listeners, 1)

	assert.Equal(t, 2, stub.count("DescribeListeners"))
	assert.Equal(t, 1, stub.count("DescribeTags"))
	assert.Equal(t, "front:HTTPS:443", listeners[0].GetName())
	assert.Equal(t, albListenerArn, listeners[0].GetTagValue("service"))
	assert.Equal(t, []string{tgArn}, listeners[0].ForwardTargetGroupArns())
}

// A load balancer whose listeners cannot be read would otherwise look like one without listeners.
func TestListListenersAllFailsOnOneLoadBalancer(t *testing.T) {
	repo, _ := newELBStub(t, map[string]func(http.ResponseWriter, map[string][]string){
		"DescribeLoadBalancers": lbPage(albArn, nlbArn),
		"DescribeListeners":     func(w http.ResponseWriter, _ map[string][]string) { fail(w, "AccessDenied") },
	})

	listeners, err := repo.ListListenersAll()
	require.Error(t, err)
	assert.Nil(t, listeners)
}

func TestListListenersByInputNil(t *testing.T) {
	repo, stub := newELBStub(t, nil)

	listeners, err := repo.ListListenersByInput(nil)
	require.NoError(t, err)
	assert.Empty(t, listeners)
	assert.Zero(t, stub.count("DescribeListeners"))
}

func lbArnN(i int) string {
	return fmt.Sprintf("arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/app/lb-%02d/%016x", i, i)
}

// loadBalancerPages serves 22 load balancers over two pages.
func loadBalancerPages(w http.ResponseWriter, form map[string][]string) {
	from, to, next := 1, 12, "<NextMarker>page2</NextMarker>"
	if len(form["Marker"]) > 0 && form["Marker"][0] == "page2" {
		from, to, next = 13, 22, ""
	}

	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, `<member><LoadBalancerArn>%s</LoadBalancerArn><LoadBalancerName>lb-%02d</LoadBalancerName></member>`, lbArnN(i), i)
	}

	respond(w, "DescribeLoadBalancers", "<LoadBalancers>"+b.String()+"</LoadBalancers>"+next)
}

// The listing used to stop at the first DescribeLoadBalancers page and read tags one load balancer at a time.
func TestListLoadBalancersAllPaginatesAndBatchesTags(t *testing.T) {
	repo, stub := newELBStub(t, map[string]func(http.ResponseWriter, map[string][]string){
		"DescribeLoadBalancers": loadBalancerPages,
		"DescribeTags":          tagsFor,
	})

	balancers, err := repo.ListLoadBalancersAll()
	require.NoError(t, err)
	require.Len(t, balancers, 22, "every page is read")

	assert.Equal(t, 2, stub.count("DescribeLoadBalancers"))
	assert.Equal(t, 2, stub.count("DescribeTags"), "22 ARNs at 20 per DescribeTags call")

	for i, lb := range balancers {
		assert.Equal(t, fmt.Sprintf("lb-%02d", i+1), lb.GetName())
		assert.Equal(t, lbArnN(i+1), lb.GetTagValue("service"))
	}
}

// A load balancer returned without its tags would read as untagged.
func TestListLoadBalancersFailsOnTags(t *testing.T) {
	repo, _ := newELBStub(t, map[string]func(http.ResponseWriter, map[string][]string){
		"DescribeLoadBalancers": loadBalancerPages,
		"DescribeTags":          func(w http.ResponseWriter, _ map[string][]string) { fail(w, "AccessDenied") },
	})

	balancers, err := repo.ListLoadBalancersAll()
	require.Error(t, err)
	assert.Nil(t, balancers)
}
