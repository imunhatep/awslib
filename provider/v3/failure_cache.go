package v3

import (
	"sync"
	"time"

	"github.com/imunhatep/awslib/provider/types"
)

// DefaultClientFailureTTL bounds how long a client-creation failure is
// remembered.
//
// The value trades two costs against each other. Too short, and a wide
// multi-region sweep re-probes every disabled region on every request — the
// problem this cache exists to solve. Too long, and a recovered credential is
// not noticed: an expired SSO session fails client creation the same way a
// disabled region does, so caching that outcome indefinitely would keep the pool
// broken after the operator logged back in. Five minutes collapses the repeats
// within a burst of queries while keeping the recovery window short.
const DefaultClientFailureTTL = 5 * time.Minute

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
// pass a result through unconditionally.
func (c *FailureCache) Add(accountID types.AwsAccountID, region types.AwsRegion, err error) {
	if c == nil || err == nil {
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
