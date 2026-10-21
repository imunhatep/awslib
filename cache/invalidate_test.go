package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/cache/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type item struct{ Name string }

// readOnlyHandler cannot evict: Invalidate must skip it rather than fail.
type readOnlyHandler struct{ hits map[string]item }

func (h readOnlyHandler) Type() string { return "readonly" }
func (h readOnlyHandler) Read(key string, data interface{}) bool {
	v, ok := h.hits[key]
	if ok {
		*data.(*item) = v
	}
	return ok
}
func (h readOnlyHandler) Write(key string, data interface{}) error {
	if v, ok := data.(item); ok {
		h.hits[key] = v
	} else {
		h.hits[key] = *data.(*item)
	}
	return nil
}

func newHandlers(t *testing.T) []cache.HandlerInterface {
	t.Helper()

	bc, err := bigcache.New(context.Background(), bigcache.DefaultConfig(time.Minute))
	require.NoError(t, err)

	file, err := handlers.NewInFile(t.TempDir(), time.Minute)
	require.NoError(t, err)

	return []cache.HandlerInterface{handlers.NewInMemory(bc), file}
}

func TestInvalidateEvictsByPrefixWithinNamespace(t *testing.T) {
	dc := cache.NewDataCache().WithHandlers(newHandlers(t)...)
	ns := dc.WithNamespace("1:eu-central-1:cloudcontrol")
	other := dc.WithNamespace("1:eu-west-1:cloudcontrol")

	require.NoError(t, ns.Write(cache.Key("ListResourcesByType", "a"), item{"a"}))
	require.NoError(t, ns.Write(cache.Key("ListResourcesByInput", "b", true), item{"b"}))
	require.NoError(t, ns.Write(cache.Key("DescribeSomething", "c"), item{"c"}))
	require.NoError(t, other.Write(cache.Key("ListResourcesByType", "a"), item{"other"}))

	require.NoError(t, ns.Invalidate("ListResources"))

	var got item
	assert.False(t, ns.Read(cache.Key("ListResourcesByType", "a"), &got), "evicted")
	assert.False(t, ns.Read(cache.Key("ListResourcesByInput", "b", true), &got), "evicted")
	assert.True(t, ns.Read(cache.Key("DescribeSomething", "c"), &got), "different method prefix survives")
	assert.True(t, other.Read(cache.Key("ListResourcesByType", "a"), &got), "other namespace survives")
}

func TestInvalidateSkipsHandlersThatCannotEvict(t *testing.T) {
	ro := readOnlyHandler{hits: map[string]item{}}
	dc := cache.NewDataCache().WithHandlers(ro).WithNamespace("ns")

	require.NoError(t, dc.Write("ListX", item{"x"}))
	require.NoError(t, dc.Invalidate("List"))

	var got item
	assert.True(t, dc.Read("ListX", &got), "a handler without DeletePrefix keeps its entry")
}
