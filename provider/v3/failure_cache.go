package v3

import (
	"strings"
	"sync"
	"time"

	"github.com/imunhatep/awslib/provider/types"
	"github.com/rs/zerolog/log"
)

// DefaultClientFailureTTL bounds how long a client-creation failure is
// remembered. It is set on the same order as a typical resource cache TTL, and
// for the same reason: both answer "has anything changed since we last looked?",
// and a short window here would be wasted, because a region an account has not
// enabled does not become enabled between two queries minutes apart.
//
// This is only the library default. A consumer that already has a cache TTL of
// its own should pass it via WithFailureTTL so the two age on one clock instead
// of leaving a second, hidden window — aws-mcp-go wires its --cache-ttl through.
//
// A TTL this long is only safe because credential failures are excluded from the
// cache entirely (see credentialFailure). Without that exclusion an expired SSO
// session would poison every region at once, and the pool would stay broken for
// the whole TTL after the operator logged back in.
const DefaultClientFailureTTL = 3 * time.Hour

// credentialFailure reports whether an error is about the caller's credentials
// rather than the target region.
//
// This distinction is what makes a long TTL safe. A region the account has not
// enabled, or one whose endpoint does not route, is a stable fact worth
// remembering for hours. An expired or missing credential is transient, fails
// identically for every region at once, and is fixed by re-authenticating — so
// caching it would turn a 30-second `aws sso login` into a TTL-long outage.
//
// Deliberately absent from the list: InvalidClientTokenId. That is what STS
// returns for a region the account has not opted into, which is the single most
// common thing this cache exists to remember. It can also mean a rotated static
// key, so a static-credential deployment may cache one round of failures until
// the TTL lapses — the accepted cost of keeping the common case fast.
func credentialFailure(err error) bool {
	if err == nil {
		return false
	}

	haystack := strings.ToLower(err.Error())

	markers := []string{
		"expired",                        // ExpiredToken/ExpiredTokenException/RequestExpired, expired SSO token
		"failed to refresh",              // cached SSO token could not be refreshed
		"invalidgrant",                   // SSO refresh rejected
		"unauthorizedexception",          // SSO GetRoleCredentials 401
		"nocredentialproviders",          // nothing in the chain resolved
		"failed to retrieve credentials", // provider chain gave up
		"no valid credential",
	}

	for _, marker := range markers {
		if strings.Contains(haystack, marker) {
			return true
		}
	}

	return false
}

// FailureCache remembers per (account, region) client-creation failures so a
// region that cannot produce a client is not re-probed on every request.
//
// Successful clients are cached for the pool's lifetime; failures used not to be
// cached at all. A region that is disabled for the account costs a rejected STS
// call every time, and one whose endpoint does not route costs a full TCP
// timeout every time — paid again on every query, for every such region.
//
// A zero FailureCache is not usable; construct it with NewFailureCache.
type FailureCache struct {
	mx      sync.Mutex
	ttl     time.Duration
	entries map[types.AwsAccountID]map[types.AwsRegion]failureEntry
}

type failureEntry struct {
	err error
	at  time.Time
}

// NewFailureCache returns a cache holding failures for ttl. A non-positive ttl
// selects DefaultClientFailureTTL.
func NewFailureCache(ttl time.Duration) *FailureCache {
	if ttl <= 0 {
		ttl = DefaultClientFailureTTL
	}

	return &FailureCache{
		ttl:     ttl,
		entries: map[types.AwsAccountID]map[types.AwsRegion]failureEntry{},
	}
}

// Err returns the remembered failure for this account and region while it is
// still fresh, or nil. A nil receiver reports no failure, so a pool constructed
// without a cache keeps its previous behaviour.
func (c *FailureCache) Err(accountID types.AwsAccountID, region types.AwsRegion) error {
	if c == nil {
		return nil
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	regions, ok := c.entries[accountID]
	if !ok {
		return nil
	}

	entry, ok := regions[region]
	if !ok {
		return nil
	}

	if time.Since(entry.at) > c.ttl {
		delete(regions, region)
		return nil
	}

	return entry.err
}

// Add remembers a client-creation failure. A nil error is ignored so callers can
// pass a result through unconditionally, and a credential failure is ignored
// because it says nothing about the region and would outlive the fix for it.
func (c *FailureCache) Add(accountID types.AwsAccountID, region types.AwsRegion, err error) {
	if c == nil || err == nil {
		return
	}

	if credentialFailure(err) {
		log.Debug().Err(err).
			Stringer("accountID", accountID).
			Stringer("region", region).
			Msg("[FailureCache.Add] credential failure, not cached so re-authenticating takes effect immediately")

		return
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	if _, ok := c.entries[accountID]; !ok {
		c.entries[accountID] = map[types.AwsRegion]failureEntry{}
	}

	c.entries[accountID][region] = failureEntry{err: err, at: time.Now()}
}

// Forget drops any remembered failure, so a pair that starts working is not held
// back by a stale entry for the rest of its TTL.
func (c *FailureCache) Forget(accountID types.AwsAccountID, region types.AwsRegion) {
	if c == nil {
		return
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	if regions, ok := c.entries[accountID]; ok {
		delete(regions, region)
	}
}
