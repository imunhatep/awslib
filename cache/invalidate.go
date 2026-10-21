package cache

import (
	"github.com/rs/zerolog/log"
)

// InvalidatorInterface is the optional eviction side of a cache handler.
//
// It is a separate interface rather than a method on HandlerInterface so that
// adding it did not break external handler implementations. DataCache.Invalidate
// skips handlers that do not implement it, with a warning, because a handler
// that cannot forget an entry keeps serving it until its TTL lapses.
type InvalidatorInterface interface {
	// DeletePrefix removes every entry whose key starts with prefix. A prefix
	// matching nothing is not an error.
	DeletePrefix(prefix string) error
}

// Invalidate evicts every entry in this namespace whose method-name prefix
// starts with prefix, from every handler that can evict.
//
// Cache keys are "<namespace>:<method>-<hash of params>", and the hash is one
// way, so there is no evicting "the entries for resource type X" without knowing
// every parameter set that was ever cached. Callers that write to a resource
// therefore evict by method-name prefix — wider than strictly needed, within one
// namespace — because a stale list that still shows a deleted resource is worse
// than a refetch of a neighbouring type.
func (c *DataCache) Invalidate(prefix string) error {
	cacheKey := c.getKey(prefix)
	log.Debug().Str("prefix", cacheKey).Msg("[DataCache.Invalidate] evict cache entries")

	var firstErr error
	for _, handler := range c.handlers {
		invalidator, ok := handler.(InvalidatorInterface)
		if !ok {
			log.Warn().
				Str("handler", handler.Type()).
				Str("prefix", cacheKey).
				Msg("[DataCache.Invalidate] handler cannot evict, entries stay until their TTL lapses")
			continue
		}

		if err := invalidator.DeletePrefix(cacheKey); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
