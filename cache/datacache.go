package cache

import (
	"fmt"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

type HandlerInterface interface {
	Type() string
	Read(string, interface{}) bool
	Write(string, interface{}) error
}

type DataCache struct {
	namespace string
	handlers  []HandlerInterface
}

func NewDataCache() *DataCache {
	return &DataCache{}
}

func (c *DataCache) WithNamespace(namespace string) *DataCache {
	return &DataCache{
		handlers:  c.handlers,
		namespace: namespace,
	}
}

func (c *DataCache) WithHandlers(handlers ...HandlerInterface) *DataCache {
	return &DataCache{
		namespace: c.namespace,
		handlers:  append(c.handlers, handlers...),
	}
}

func (c *DataCache) Read(name string, data interface{}) bool {
	cacheKey := c.getKey(name)
	log.Debug().Str("key", cacheKey).Msg("[DataCache.Read] read cache")

	for _, handler := range c.handlers {
		if metrics.AwsMetricsEnabled {
			metrics.AwsResourceCacheRead.WithLabelValues(c.namespace, name, handler.Type()).Inc()
		}
		if handler.Read(cacheKey, data) {
			if metrics.AwsMetricsEnabled {
				metrics.AwsResourceCacheHit.WithLabelValues(c.namespace, name, handler.Type()).Inc()
			}
			return true
		}
	}

	return false
}

func (c *DataCache) Write(name string, data interface{}) error {
	var errors []error

	cacheKey := c.getKey(name)
	log.Debug().Str("key", cacheKey).Msg("[DataCache.Write] write cache")

	for _, handler := range c.handlers {
		if metrics.AwsMetricsEnabled {
			metrics.AwsResourceCacheWrite.WithLabelValues(c.namespace, name, handler.Type()).Inc()
		}

		if err := handler.Write(cacheKey, data); err != nil {
			errors = append(errors, err)
			if metrics.AwsMetricsEnabled {
				metrics.AwsResourceCacheError.WithLabelValues(c.namespace, name, handler.Type()).Inc()
			}
		}
	}

	return slice.Head(errors).OrEmpty()
}

func (c *DataCache) getKey(name string) string {
	return fmt.Sprintf("%s:%s", c.namespace, name)
}
