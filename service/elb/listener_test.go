package elb

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/stretchr/testify/assert"
)

const (
	albListenerArn = "arn:aws:elasticloadbalancing:eu-west-1:123456789012:listener/app/front/50dc6c495c0c9188/f2f7dc8efc522ab2"
	gwlbArn        = "arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/gwy/inspect/0e9f8d7c6b5a4f3e"
	tg2Arn         = "arn:aws:elasticloadbalancing:eu-west-1:123456789012:targetgroup/api/1234567890abcdef"
)

func albListener() types.Listener {
	return types.Listener{
		ListenerArn:     aws.String(albListenerArn),
		LoadBalancerArn: aws.String(albArn),
		Protocol:        types.ProtocolEnumHttps,
		Port:            aws.Int32(443),
		SslPolicy:       aws.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
		Certificates:    []types.Certificate{{CertificateArn: aws.String("arn:aws:acm:eu-west-1:123456789012:certificate/abc")}},
		DefaultActions: []types.Action{
			{
				Type:  types.ActionTypeEnumForward,
				Order: aws.Int32(2),
				ForwardConfig: &types.ForwardActionConfig{TargetGroups: []types.TargetGroupTuple{
					{TargetGroupArn: aws.String(tgArn), Weight: aws.Int32(90)},
					{TargetGroupArn: aws.String(tg2Arn), Weight: aws.Int32(10)},
				}},
				TargetGroupArn: aws.String(tgArn),
			},
			{
				Type:                   types.ActionTypeEnumAuthenticateOidc,
				Order:                  aws.Int32(1),
				AuthenticateOidcConfig: &types.AuthenticateOidcActionConfig{Issuer: aws.String("https://idp")},
			},
		},
	}
}

func TestListener(t *testing.T) {
	l := NewListener(fakeClient{}, albListener(), []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}})

	assert.Equal(t, "front:HTTPS:443", l.GetName())
	assert.Equal(t, albListenerArn, l.GetArn())
	assert.Equal(t, cfg.ResourceTypeListenerV2, l.GetType())
	assert.Equal(t, "prod", l.GetTagValue("env"))
	assert.Equal(t, "HTTPS", l.GetProtocol())
	assert.Equal(t, int32(443), l.GetPort())
	assert.Equal(t, "ELBSecurityPolicy-TLS13-1-2-2021-06", l.GetSslPolicy())
	assert.Equal(t, []string{"arn:aws:acm:eu-west-1:123456789012:certificate/abc"}, l.CertificateArns())
	assert.Equal(t, []string{"authenticate-oidc", "forward"}, l.DefaultActionTypes(), "evaluation order, not slice order")
	assert.Equal(t, []string{tgArn, tg2Arn}, l.ForwardTargetGroupArns(), "single and weighted forms, deduplicated")
	assert.Empty(t, l.RedirectTarget())
	assert.Equal(t, albArn, l.GetLoadBalancerArn())
	assert.Equal(t, "front", l.GetLoadBalancerName())
	assert.Equal(t, "application", l.GetLoadBalancerType())
}

func TestListenerRedirect(t *testing.T) {
	l := NewListener(fakeClient{}, types.Listener{
		ListenerArn:     aws.String(albListenerArn),
		LoadBalancerArn: aws.String(albArn),
		Protocol:        types.ProtocolEnumHttp,
		Port:            aws.Int32(80),
		DefaultActions: []types.Action{{
			Type: types.ActionTypeEnumRedirect,
			RedirectConfig: &types.RedirectActionConfig{
				Protocol:   aws.String("HTTPS"),
				Host:       aws.String("#{host}"),
				Port:       aws.String("443"),
				Path:       aws.String("/#{path}"),
				Query:      aws.String("#{query}"),
				StatusCode: types.RedirectActionStatusCodeEnumHttp301,
			},
		}},
	}, nil)

	assert.Equal(t, "front:HTTP:80", l.GetName())
	assert.Equal(t, "https://#{host}:443/#{path}?#{query} (HTTP_301)", l.RedirectTarget())
	assert.Equal(t, []string{"redirect"}, l.DefaultActionTypes())
	assert.Empty(t, l.ForwardTargetGroupArns())
	assert.Empty(t, l.CertificateArns())
}

// Gateway load balancer listeners carry no protocol or port.
func TestGatewayListenerName(t *testing.T) {
	l := NewListener(fakeClient{}, types.Listener{
		ListenerArn:     aws.String("arn:aws:elasticloadbalancing:eu-west-1:123456789012:listener/gwy/inspect/0e9f8d7c6b5a4f3e/99aa88bb77cc66dd"),
		LoadBalancerArn: aws.String(gwlbArn),
	}, nil)

	assert.Equal(t, "inspect:99aa88bb77cc66dd", l.GetName())
	assert.Equal(t, "gateway", l.GetLoadBalancerType())
}

func TestParseLoadBalancerArn(t *testing.T) {
	for _, tc := range []struct{ arn, lbType, name string }{
		{albArn, "application", "front"},
		{nlbArn, "network", "edge"},
		{albListenerArn, "application", "front"},
		{"not-an-arn", "", ""},
		{"arn:aws:elasticloadbalancing:eu-west-1:123456789012:loadbalancer/classic", "", ""},
	} {
		lbType, name := parseLoadBalancerArn(tc.arn)
		assert.Equal(t, tc.lbType, lbType, tc.arn)
		assert.Equal(t, tc.name, name, tc.arn)
	}
}
