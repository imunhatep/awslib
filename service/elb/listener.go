package elb

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/imunhatep/awslib/service"
)

type Listener struct {
	service.AbstractResource
	types.Listener
	Tags []types.Tag
}

func NewListener(client AwsClient, l types.Listener, tags []types.Tag) Listener {
	lArn, _ := arn.Parse(aws.ToString(l.ListenerArn))

	return Listener{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        "",
			ARN:       &lArn,
			CreatedAt: time.Unix(0, 0),
			Type:      cfg.ResourceTypeListenerV2,
		},
		Listener: l,
		Tags:     tags,
	}
}

// GetName renders "<lb-name>:<protocol>:<port>". Gateway load balancer
// listeners carry neither, so they are named "<lb-name>:<listener-id>".
func (e Listener) GetName() string {
	lbName := e.GetLoadBalancerName()

	if e.Protocol == "" && e.Port == nil {
		return lbName + ":" + listenerID(aws.ToString(e.ListenerArn))
	}

	return fmt.Sprintf("%s:%s:%d", lbName, e.Protocol, aws.ToInt32(e.Port))
}

func (e Listener) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Listener) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}

func (e Listener) GetProtocol() string {
	return string(e.Protocol)
}

func (e Listener) GetPort() int32 {
	return aws.ToInt32(e.Port)
}

func (e Listener) GetSslPolicy() string {
	return aws.ToString(e.SslPolicy)
}

// CertificateArns lists the listener's certificates as DescribeListeners
// returns them, which is the default certificate only. Additional SNI
// certificates need DescribeListenerCertificates and are not read.
func (e Listener) CertificateArns() []string {
	arns := make([]string, 0, len(e.Certificates))
	for _, c := range e.Certificates {
		if v := aws.ToString(c.CertificateArn); v != "" {
			arns = append(arns, v)
		}
	}

	return arns
}

// DefaultActionTypes lists the default action types in evaluation order,
// e.g. ["authenticate-oidc", "forward"].
func (e Listener) DefaultActionTypes() []string {
	actions := e.orderedActions()
	out := make([]string, 0, len(actions))
	for _, a := range actions {
		out = append(out, string(a.Type))
	}

	return out
}

// ForwardTargetGroupArns lists the target groups the default actions forward
// to, from both the single TargetGroupArn and a weighted ForwardConfig,
// without duplicates.
func (e Listener) ForwardTargetGroupArns() []string {
	var out []string
	add := func(v *string) {
		if s := aws.ToString(v); s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}

	for _, a := range e.orderedActions() {
		if a.Type != types.ActionTypeEnumForward {
			continue
		}

		add(a.TargetGroupArn)
		if a.ForwardConfig != nil {
			for _, tg := range a.ForwardConfig.TargetGroups {
				add(tg.TargetGroupArn)
			}
		}
	}

	return out
}

// RedirectTarget renders the default redirect action as
// "<protocol>://<host>:<port><path>?<query> (<status>)", keeping the #{...}
// placeholders AWS substitutes from the request. Empty when the default
// actions do not redirect.
func (e Listener) RedirectTarget() string {
	for _, a := range e.orderedActions() {
		if a.Type != types.ActionTypeEnumRedirect || a.RedirectConfig == nil {
			continue
		}

		rc := a.RedirectConfig
		path := aws.ToString(rc.Path)
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		target := fmt.Sprintf("%s://%s:%s%s",
			strings.ToLower(aws.ToString(rc.Protocol)), aws.ToString(rc.Host), aws.ToString(rc.Port), path)
		if q := aws.ToString(rc.Query); q != "" {
			target += "?" + q
		}

		if rc.StatusCode != "" {
			target += " (" + string(rc.StatusCode) + ")"
		}

		return target
	}

	return ""
}

func (e Listener) GetLoadBalancerArn() string {
	return aws.ToString(e.LoadBalancerArn)
}

// GetLoadBalancerName is the parent load balancer's name, parsed from its ARN.
func (e Listener) GetLoadBalancerName() string {
	_, name := parseLoadBalancerArn(e.GetLoadBalancerArn())
	return name
}

// GetLoadBalancerType is "application", "network" or "gateway", parsed from
// the parent load balancer's ARN.
func (e Listener) GetLoadBalancerType() string {
	lbType, _ := parseLoadBalancerArn(e.GetLoadBalancerArn())
	return lbType
}

func (e Listener) orderedActions() []types.Action {
	actions := slices.Clone(e.DefaultActions)
	slices.SortStableFunc(actions, func(a, b types.Action) int {
		return int(aws.ToInt32(a.Order)) - int(aws.ToInt32(b.Order))
	})

	return actions
}

var loadBalancerTypes = map[string]string{
	"app": "application",
	"net": "network",
	"gwy": "gateway",
}

// parseLoadBalancerArn extracts the type and name from a load balancer ARN
// (resource "loadbalancer/app/<name>/<id>") or a listener ARN
// (resource "listener/app/<name>/<lb-id>/<id>").
func parseLoadBalancerArn(value string) (lbType, name string) {
	parsed, err := arn.Parse(value)
	if err != nil {
		return "", ""
	}

	parts := strings.Split(parsed.Resource, "/")
	if len(parts) < 3 {
		return "", ""
	}

	lbType, ok := loadBalancerTypes[parts[1]]
	if !ok {
		lbType = parts[1]
	}

	return lbType, parts[2]
}

// listenerID is the last segment of a listener ARN.
func listenerID(listenerArn string) string {
	return listenerArn[strings.LastIndex(listenerArn, "/")+1:]
}
