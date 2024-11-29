package handlers

import (
	"bytes"
	"encoding/gob"
	"github.com/allegro/bigcache/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"sync"
)

const CacheTypeMemory = "memory"

type InMemory struct {
	cache *bigcache.BigCache
	mx    sync.Mutex
}

func NewInMemory(cache *bigcache.BigCache) *InMemory {
	return &InMemory{cache: cache}
}

func (h *InMemory) Type() string {
	return CacheTypeMemory
}

func (h *InMemory) Read(name string, data interface{}) bool {
	h.mx.Lock()
	defer h.mx.Unlock()

	logger := h.getLogger(name)

	cache, err := h.cache.Get(name)
	if err != nil {
		logger.Debug().Err(err).Msg("[InMemory.Read] cache MISS")
		return false
	}

	err = gob.NewDecoder(bytes.NewBuffer(cache)).Decode(data)
	if err != nil {
		logger.Error().Err(err).Msg("[InMemory.Read] cache decode fail")
		return false
	}

	logger.Debug().Msg("[InMemory.Read] cache read success")

	return true
}

func (h *InMemory) Write(name string, data interface{}) error {
	h.mx.Lock()
	defer h.mx.Unlock()

	logger := h.getLogger(name)

	store := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(store)
	if err := encoder.Encode(data); err != nil {
		logger.Error().Err(err).Msg("[InMemory.Write] cache encode fail")
		return err
	}

	logger.Debug().Msg("[InMemory.Write] cache write success")

	return h.cache.Set(name, store.Bytes())
}

func (h *InMemory) getLogger(name string) zerolog.Logger {
	return log.
		With().
		Str("key", name).
		Str("store", CacheTypeMemory).
		Logger()
}
