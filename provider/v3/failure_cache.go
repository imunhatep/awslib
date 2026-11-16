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

// NetworkFailureTTL caps how long a connection failure (ConnectFailure) is
// remembered, whatever the cache's TTL. Long enough that one sweep, and the
// follow-up queries an agent makes right after it, do not wait on a dead
// endpoint again; short enough that a blip on the caller's own network — a VPN
// reconnecting, a laptop waking up, which fail every region at once — does not
// hide healthy regions for hours.
const NetworkFailureTTL = 10 * time.Minute

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
	ttl time.Duration
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

	if time.Since(entry.at) > entry.ttl {
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

	c.entries[accountID][region] = failureEntry{err: err, at: time.Now(), ttl: c.entryTTL(err)}
}

// entryTTL is how long one failure is remembered. A connection failure is
// remembered for at most NetworkFailureTTL: it is the one kind a transient blip on
// the caller's own network can produce, and a remembered failure skips the pair
// without a trace in the results, so a healthy region must not stay hidden for
// the whole TTL. A region the account has not enabled stays disabled until
// someone acts, and is remembered for the full TTL.
func (c *FailureCache) entryTTL(err error) time.Duration {
	if ConnectFailure(err) && NetworkFailureTTL < c.ttl {
		return NetworkFailureTTL
	}

	return c.ttl
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

// RegionFailure reports whether an error from a request means the whole
// (account, region) pair is unusable, so every service there will fail the same
// way and the pair is worth skipping until the FailureCache TTL lapses.
//
// Two kinds qualify:
//   - the region is not enabled for the account: its endpoints reject the
//     caller's token (STS InvalidClientTokenId, EC2 AuthFailure, most other
//     services UnrecognizedClientException);
//   - its endpoints cannot be reached: the connection times out or is refused.
//
// Deliberately not a region failure: a credential failure (the pool must not
// remember it, see credentialFailure), access denied (usually one service's IAM
// grant, and caching it would hide every other service in the region), a DNS
// "no such host" (a service that does not exist in that region, not a region
// that does not answer), throttling, and the caller's own cancellation.
func RegionFailure(err error) bool {
	if err == nil || credentialFailure(err) {
		return false
	}

	haystack := strings.ToLower(err.Error())

	if strings.Contains(haystack, "no such host") {
		return false
	}

	// The region rejects the caller's token because the account has not
	// enabled it.
	for _, marker := range []string{
		"invalidclienttokenid",        // STS
		"unrecognizedclientexception", // most other services
		"authfailure",                 // EC2
	} {
		if strings.Contains(haystack, marker) {
			return true
		}
	}

	return ConnectFailure(err)
}

// ConnectFailure reports whether err is a failure to establish a connection to
// the endpoint: the dial timed out, was refused or had no route, or the TLS
// handshake timed out. Only establishing the connection counts — a read that
// times out halfway through a long paginated listing ("read tcp ...: i/o
// timeout") is a slow answer from a working endpoint, not a dead one — and no
// DNS failure counts either: "no such host" is a service absent from the region
// (see RegionFailure), and a resolver timeout is the caller's own network.
//
// A dial cut short by the caller's context is indistinguishable from one cut
// short by AwsDialTimeout; whoever reports the error must check the context.
//
// The retryer built by NewRetryer does not retry these: AwsDialTimeout is already
// orders of magnitude longer than a working endpoint takes to connect.
func ConnectFailure(err error) bool {
	if err == nil {
		return false
	}

	haystack := strings.ToLower(err.Error())

	if strings.Contains(haystack, "tls handshake timeout") {
		return true
	}

	// "dial tcp: lookup <host>: ..." is the local resolver failing, which
	// says nothing about the region, even when it is an i/o timeout.
	if !strings.Contains(haystack, "dial ") || strings.Contains(haystack, "lookup ") {
		return false
	}

	for _, marker := range []string{"i/o timeout", "connection refused", "network is unreachable", "no route to host"} {
		if strings.Contains(haystack, marker) {
			return true
		}
	}

	return false
}
