package wafv2

import (
	stderrors "errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awswaf "github.com/aws/aws-sdk-go-v2/service/wafv2"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

// ListWebACLsAll lists every web ACL this client's region serves, in full: the
// REGIONAL ones, plus the CLOUDFRONT ones from a us-east-1 client (see Scopes).
func (r *Wafv2Repository) ListWebACLsAll() ([]WebACL, error) {
	var out []WebACL

	for _, scope := range r.Scopes() {
		acls, err := r.ListWebACLsByScope(scope)
		if err != nil {
			return nil, err
		}

		out = append(out, acls...)
	}

	return out, nil
}

// ListWebACLsByScope lists the web ACLs of one scope and reads each in full,
// with tags and logging configuration.
//
// ListWebACLs carries names and IDs only, so this is 1+2N calls (GetWebACL and
// ListTagsForResource per ACL) plus one logging sweep. An ACL deleted between
// the list and its Get is skipped. An ACL whose tags cannot be read fails the
// listing rather than coming back untagged: an empty tag map reads as "this ACL
// is untagged", and tags are what ownership is answered from.
func (r *Wafv2Repository) ListWebACLsByScope(scope waftypes.Scope) ([]WebACL, error) {
	start := time.Now()

	summaries, err := r.ListWebACLSummariesByScope(scope)
	if err != nil {
		return nil, err
	}

	acls := make([]*WebACL, len(summaries))
	err = forEachBounded(len(summaries), func(i int) error {
		acl, err := r.readWebACL(scope, summaries[i].Name, summaries[i].Id)
		if stderrors.Is(err, ErrNotFound) {
			return nil
		}

		acls[i] = acl

		return err
	})
	if err != nil {
		return nil, err
	}

	logging, loggingErr := r.loggingConfigurations(scope)
	if loggingErr != nil {
		// Logging is enrichment. A role without wafv2:ListLoggingConfigurations
		// must not cost the caller the ACLs themselves; LoggingKnown stays false
		// so the gap is visible rather than reported as "logging off".
		log.Warn().Err(loggingErr).
			Str("accountID", r.client.GetAccountID().String()).
			Str("scope", string(scope)).
			Msg("[Wafv2Repository.ListWebACLsByScope] logging configurations unreadable")
	}

	out := make([]WebACL, 0, len(acls))
	for _, acl := range acls {
		if acl == nil {
			continue
		}

		if loggingErr == nil {
			acl.LoggingKnown = true
			if config, ok := logging[aws.ToString(acl.WebACL.ARN)]; ok {
				acl.Logging = &config
			}
		}

		out = append(out, *acl)
	}

	if metrics.AwsMetricsEnabled {
		r.countFetched("ListWebACLs", ccfg.ResourceTypeWafv2WebACL, len(out))
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListWebACLsByScope", ccfg.ResourceTypeWafv2WebACL)).
			Observe(time.Since(start).Seconds())
	}

	return out, nil
}

// ListWebACLSummariesByScope lists the names, IDs and ARNs of the web ACLs of
// one scope, without reading any of them. One call per 100 ACLs.
func (r *Wafv2Repository) ListWebACLSummariesByScope(scope waftypes.Scope) ([]waftypes.WebACLSummary, error) {
	return collectPages(r, "ListWebACLs", ccfg.ResourceTypeWafv2WebACL, func(marker *string) ([]waftypes.WebACLSummary, *string, error) {
		output, err := r.wafClient().ListWebACLs(r.ctx, &awswaf.ListWebACLsInput{
			Scope:      scope,
			Limit:      aws.Int32(listPageSize),
			NextMarker: marker,
		}, r.opts(scope)...)
		if err != nil {
			return nil, nil, err
		}

		return output.WebACLs, output.NextMarker, nil
	})
}

// GetWebACL reads one web ACL in full, with its tags and logging configuration.
// name and id are both required by the API. A missing ACL is ErrNotFound.
func (r *Wafv2Repository) GetWebACL(scope waftypes.Scope, name, id string) (*WebACL, error) {
	acl, err := r.readWebACL(scope, aws.String(name), aws.String(id))
	if err != nil {
		return nil, err
	}

	config, err := r.loggingConfiguration(scope, aws.ToString(acl.WebACL.ARN))
	if err != nil {
		log.Warn().Err(err).
			Str("webACL", name).
			Msg("[Wafv2Repository.GetWebACL] logging configuration unreadable")

		return acl, nil
	}

	acl.Logging = config
	acl.LoggingKnown = true

	return acl, nil
}

// GetWebACLByArn reads one web ACL in full from its ARN, which names its scope.
func (r *Wafv2Repository) GetWebACLByArn(webACLArn string) (*WebACL, error) {
	ref, err := ParseArn(webACLArn)
	if err != nil {
		return nil, err
	}

	if ref.Kind != "webacl" {
		return nil, errors.Errorf("not a web ACL ARN (kind %q): %s", ref.Kind, webACLArn)
	}

	return r.GetWebACL(ref.Scope, ref.Name, ref.Id)
}

func (r *Wafv2Repository) readWebACL(scope waftypes.Scope, name, id *string) (*WebACL, error) {
	r.countRequest("GetWebACL", ccfg.ResourceTypeWafv2WebACL)

	output, err := r.wafClient().GetWebACL(r.ctx, &awswaf.GetWebACLInput{
		Scope: scope,
		Name:  name,
		Id:    id,
	}, r.opts(scope, withRetries)...)
	if err != nil {
		r.countError("GetWebACL", ccfg.ResourceTypeWafv2WebACL)

		return nil, classifyErr(err)
	}

	if output.WebACL == nil {
		return nil, errors.New(ErrNotFound)
	}

	tags, err := r.listTags(scope, output.WebACL.ARN, ccfg.ResourceTypeWafv2WebACL)
	if err != nil {
		return nil, errors.Errorf("reading tags of web ACL %s: %w", aws.ToString(name), err)
	}

	acl := NewWebACL(r.client, scope, *output.WebACL, tags)

	return &acl, nil
}

// loggingConfigurations reads every logging configuration of a scope in one
// sweep, keyed by web ACL ARN — one call per 100 configurations instead of a
// GetLoggingConfiguration per ACL.
func (r *Wafv2Repository) loggingConfigurations(scope waftypes.Scope) (map[string]waftypes.LoggingConfiguration, error) {
	configs, err := collectPages(r, "ListLoggingConfigurations", ccfg.ResourceTypeWafv2WebACL, func(marker *string) ([]waftypes.LoggingConfiguration, *string, error) {
		output, err := r.wafClient().ListLoggingConfigurations(r.ctx, &awswaf.ListLoggingConfigurationsInput{
			Scope:      scope,
			Limit:      aws.Int32(listPageSize),
			NextMarker: marker,
		}, r.opts(scope)...)
		if err != nil {
			return nil, nil, err
		}

		return output.LoggingConfigurations, output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	out := make(map[string]waftypes.LoggingConfiguration, len(configs))
	for _, config := range configs {
		out[aws.ToString(config.ResourceArn)] = config
	}

	return out, nil
}

// loggingConfiguration reads the logging configuration of one ACL; nil when
// logging is off.
func (r *Wafv2Repository) loggingConfiguration(scope waftypes.Scope, webACLArn string) (*waftypes.LoggingConfiguration, error) {
	r.countRequest("GetLoggingConfiguration", ccfg.ResourceTypeWafv2WebACL)

	output, err := r.wafClient().GetLoggingConfiguration(r.ctx, &awswaf.GetLoggingConfigurationInput{
		ResourceArn: aws.String(webACLArn),
	}, r.opts(scope)...)
	if err != nil {
		if err = classifyErr(err); stderrors.Is(err, ErrNotFound) {
			return nil, nil
		}

		r.countError("GetLoggingConfiguration", ccfg.ResourceTypeWafv2WebACL)

		return nil, err
	}

	return output.LoggingConfiguration, nil
}
