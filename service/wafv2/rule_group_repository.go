package wafv2

import (
	stderrors "errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awswaf "github.com/aws/aws-sdk-go-v2/service/wafv2"
	waftypes "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/go-errors/errors"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// ListRuleGroupsAll lists every customer-owned rule group this client's region
// serves, in full (see Scopes).
func (r *Wafv2Repository) ListRuleGroupsAll() ([]RuleGroup, error) {
	return listAllScopes(r, r.ListRuleGroupsByScope)
}

// ListRuleGroupsByScope lists the rule groups of one scope and reads each in
// full, with tags: 1+2N calls. A group deleted between list and Get is skipped.
func (r *Wafv2Repository) ListRuleGroupsByScope(scope waftypes.Scope) ([]RuleGroup, error) {
	summaries, err := collectPages(r, "ListRuleGroups", ccfg.ResourceTypeWafv2RuleGroup, func(marker *string) ([]waftypes.RuleGroupSummary, *string, error) {
		output, err := r.wafClient().ListRuleGroups(r.ctx, &awswaf.ListRuleGroupsInput{
			Scope:      scope,
			Limit:      aws.Int32(listPageSize),
			NextMarker: marker,
		}, r.opts(scope)...)
		if err != nil {
			return nil, nil, err
		}

		return output.RuleGroups, output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	groups, err := readEach(len(summaries), func(i int) (*RuleGroup, error) {
		return r.readRuleGroup(scope, summaries[i].Name, summaries[i].Id)
	})
	if err != nil {
		return nil, err
	}

	r.countFetched("ListRuleGroups", ccfg.ResourceTypeWafv2RuleGroup, len(groups))

	return groups, nil
}

// GetRuleGroup reads one rule group in full, with its tags.
func (r *Wafv2Repository) GetRuleGroup(scope waftypes.Scope, name, id string) (*RuleGroup, error) {
	return r.readRuleGroup(scope, aws.String(name), aws.String(id))
}

func (r *Wafv2Repository) readRuleGroup(scope waftypes.Scope, name, id *string) (*RuleGroup, error) {
	r.countRequest("GetRuleGroup", ccfg.ResourceTypeWafv2RuleGroup)

	output, err := r.wafClient().GetRuleGroup(r.ctx, &awswaf.GetRuleGroupInput{
		Scope: scope,
		Name:  name,
		Id:    id,
	}, r.opts(scope, withRetries)...)
	if err != nil {
		r.countError("GetRuleGroup", ccfg.ResourceTypeWafv2RuleGroup)

		return nil, classifyErr(err)
	}

	if output.RuleGroup == nil {
		return nil, errors.New(ErrNotFound)
	}

	tags, err := r.listTags(scope, output.RuleGroup.ARN, ccfg.ResourceTypeWafv2RuleGroup)
	if err != nil {
		return nil, errors.Errorf("reading tags of rule group %s: %w", aws.ToString(name), err)
	}

	group := NewRuleGroup(r.client, scope, *output.RuleGroup, tags)

	return &group, nil
}

// ListIPSetsAll lists every IP set this client's region serves, addresses
// included (see Scopes).
func (r *Wafv2Repository) ListIPSetsAll() ([]IPSet, error) {
	return listAllScopes(r, r.ListIPSetsByScope)
}

// ListIPSetsByScope lists the IP sets of one scope and reads each in full.
func (r *Wafv2Repository) ListIPSetsByScope(scope waftypes.Scope) ([]IPSet, error) {
	summaries, err := collectPages(r, "ListIPSets", ccfg.ResourceTypeWafv2IPSet, func(marker *string) ([]waftypes.IPSetSummary, *string, error) {
		output, err := r.wafClient().ListIPSets(r.ctx, &awswaf.ListIPSetsInput{
			Scope:      scope,
			Limit:      aws.Int32(listPageSize),
			NextMarker: marker,
		}, r.opts(scope)...)
		if err != nil {
			return nil, nil, err
		}

		return output.IPSets, output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	sets, err := readEach(len(summaries), func(i int) (*IPSet, error) {
		r.countRequest("GetIPSet", ccfg.ResourceTypeWafv2IPSet)

		output, err := r.wafClient().GetIPSet(r.ctx, &awswaf.GetIPSetInput{
			Scope: scope,
			Name:  summaries[i].Name,
			Id:    summaries[i].Id,
		}, r.opts(scope, withRetries)...)
		if err != nil {
			r.countError("GetIPSet", ccfg.ResourceTypeWafv2IPSet)

			return nil, classifyErr(err)
		}

		if output.IPSet == nil {
			return nil, errors.New(ErrNotFound)
		}

		tags, err := r.listTags(scope, output.IPSet.ARN, ccfg.ResourceTypeWafv2IPSet)
		if err != nil {
			return nil, errors.Errorf("reading tags of IP set %s: %w", aws.ToString(summaries[i].Name), err)
		}

		set := NewIPSet(r.client, scope, *output.IPSet, tags)

		return &set, nil
	})
	if err != nil {
		return nil, err
	}

	r.countFetched("ListIPSets", ccfg.ResourceTypeWafv2IPSet, len(sets))

	return sets, nil
}

// ListRegexPatternSetsAll lists every regex pattern set this client's region
// serves, patterns included (see Scopes).
func (r *Wafv2Repository) ListRegexPatternSetsAll() ([]RegexPatternSet, error) {
	return listAllScopes(r, r.ListRegexPatternSetsByScope)
}

// ListRegexPatternSetsByScope lists the regex pattern sets of one scope and
// reads each in full.
func (r *Wafv2Repository) ListRegexPatternSetsByScope(scope waftypes.Scope) ([]RegexPatternSet, error) {
	summaries, err := collectPages(r, "ListRegexPatternSets", ccfg.ResourceTypeWafv2RegexPatternSet, func(marker *string) ([]waftypes.RegexPatternSetSummary, *string, error) {
		output, err := r.wafClient().ListRegexPatternSets(r.ctx, &awswaf.ListRegexPatternSetsInput{
			Scope:      scope,
			Limit:      aws.Int32(listPageSize),
			NextMarker: marker,
		}, r.opts(scope)...)
		if err != nil {
			return nil, nil, err
		}

		return output.RegexPatternSets, output.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}

	sets, err := readEach(len(summaries), func(i int) (*RegexPatternSet, error) {
		r.countRequest("GetRegexPatternSet", ccfg.ResourceTypeWafv2RegexPatternSet)

		output, err := r.wafClient().GetRegexPatternSet(r.ctx, &awswaf.GetRegexPatternSetInput{
			Scope: scope,
			Name:  summaries[i].Name,
			Id:    summaries[i].Id,
		}, r.opts(scope, withRetries)...)
		if err != nil {
			r.countError("GetRegexPatternSet", ccfg.ResourceTypeWafv2RegexPatternSet)

			return nil, classifyErr(err)
		}

		if output.RegexPatternSet == nil {
			return nil, errors.New(ErrNotFound)
		}

		tags, err := r.listTags(scope, output.RegexPatternSet.ARN, ccfg.ResourceTypeWafv2RegexPatternSet)
		if err != nil {
			return nil, errors.Errorf("reading tags of regex pattern set %s: %w", aws.ToString(summaries[i].Name), err)
		}

		set := NewRegexPatternSet(r.client, scope, *output.RegexPatternSet, tags)

		return &set, nil
	})
	if err != nil {
		return nil, err
	}

	r.countFetched("ListRegexPatternSets", ccfg.ResourceTypeWafv2RegexPatternSet, len(sets))

	return sets, nil
}

// listAllScopes concatenates a by-scope listing over every scope this client
// covers.
func listAllScopes[T any](r *Wafv2Repository, byScope func(waftypes.Scope) ([]T, error)) ([]T, error) {
	var out []T

	for _, scope := range r.Scopes() {
		items, err := byScope(scope)
		if err != nil {
			return nil, err
		}

		out = append(out, items...)
	}

	return out, nil
}

// readEach runs the per-object reads of a listing with bounded concurrency,
// keeping list order and skipping objects deleted since the list (ErrNotFound).
func readEach[T any](n int, read func(i int) (*T, error)) ([]T, error) {
	found := make([]*T, n)

	err := forEachBounded(n, func(i int) error {
		item, err := read(i)
		if stderrors.Is(err, ErrNotFound) {
			return nil
		}

		found[i] = item

		return err
	})
	if err != nil {
		return nil, err
	}

	out := make([]T, 0, n)
	for _, item := range found {
		if item != nil {
			out = append(out, *item)
		}
	}

	return out, nil
}

// collectPages drives a WAFv2 NextMarker loop. WAFv2 has no SDK paginators.
// The loop also stops on an empty page or a repeated marker, so a service that
// keeps handing back a marker cannot spin it forever.
func collectPages[T any](r *Wafv2Repository, method string, resourceType cfg.ResourceType, page func(marker *string) ([]T, *string, error)) ([]T, error) {
	var (
		out    []T
		marker *string
	)

	for {
		r.countRequest(method, resourceType)

		items, next, err := page(marker)
		if err != nil {
			r.countError(method, resourceType)

			return nil, classifyErr(err)
		}

		out = append(out, items...)

		if aws.ToString(next) == "" || len(items) == 0 || aws.ToString(next) == aws.ToString(marker) {
			return out, nil
		}

		marker = next
	}
}
