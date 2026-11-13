package proxy

import (
	"context"

	"github.com/imunhatep/awslib/cache"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/wafv2"
	"github.com/imunhatep/gocollection/slice"
)

// FindWafv2WebACLs returns the web ACLs this client's region serves, in full —
// REGIONAL ones, plus the CLOUDFRONT ones from a us-east-1 client.
func FindWafv2WebACLs(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := wafv2.NewWafv2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListWebACLsAll()
		return slice.Map(items, cast[wafv2.WebACL]), err
	}
	items, err := repo.ListWebACLsAll()
	return slice.Map(items, cast[wafv2.WebACL]), err
}

// FindWafv2RuleGroups returns the customer-owned rule groups this client's
// region serves, in full.
func FindWafv2RuleGroups(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := wafv2.NewWafv2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListRuleGroupsAll()
		return slice.Map(items, cast[wafv2.RuleGroup]), err
	}
	items, err := repo.ListRuleGroupsAll()
	return slice.Map(items, cast[wafv2.RuleGroup]), err
}

// FindWafv2IPSets returns the IP sets this client's region serves, addresses
// included.
func FindWafv2IPSets(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := wafv2.NewWafv2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListIPSetsAll()
		return slice.Map(items, cast[wafv2.IPSet]), err
	}
	items, err := repo.ListIPSetsAll()
	return slice.Map(items, cast[wafv2.IPSet]), err
}

// FindWafv2RegexPatternSets returns the regex pattern sets this client's region
// serves, patterns included.
func FindWafv2RegexPatternSets(ctx context.Context, client *v3.Client, dc *cache.DataCache) ([]service.ResourceInterface, error) {
	repo := wafv2.NewWafv2Repository(ctx, client)
	if dc != nil {
		items, err := repo.WithCache(dc).ListRegexPatternSetsAll()
		return slice.Map(items, cast[wafv2.RegexPatternSet]), err
	}
	items, err := repo.ListRegexPatternSetsAll()
	return slice.Map(items, cast[wafv2.RegexPatternSet]), err
}
