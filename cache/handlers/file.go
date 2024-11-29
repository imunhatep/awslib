package handlers

import (
	"encoding/gob"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
	"os"
	"strings"
	"sync"
	"time"
)

const CacheTypeFile = "file"

type InFile struct {
	cacheDir string
	ttl      time.Duration
	mx       sync.Mutex
}

func NewInFile(cacheDir string, ttl time.Duration) (*InFile, error) {
	log.Debug().Str("cacheDir", cacheDir).Dur("ttl", ttl).Msg("[NewInFile] new")

	cacheDir = strings.TrimRight(cacheDir, "/")
	if err := unix.Access(cacheDir, unix.W_OK); err != nil {
		return nil, errors.New(err)
	}

	handler := &InFile{
		cacheDir: cacheDir,
		ttl:      ttl,
	}

	return handler, nil
}

func (h *InFile) Type() string {
	return CacheTypeFile
}

func (h *InFile) Read(name string, data interface{}) bool {
	h.mx.Lock()
	defer h.mx.Unlock()

	// logger with struct data
	logger := h.getLogger(name)

	// check cache ttl
	if expired := h.isExpired(name); expired {
		logger.Debug().Msg("[InFile.Read] cache expired")
		return false
	}

	// filepath containing cached data
	path := h.getFilePath(name)

	// open data file
	dataFile, err := os.Open(path)
	defer dataFile.Close()

	if err != nil {
		logger.Trace().Err(err).Msg("[InFile.Read] file read error")
		return false
	}

	err = gob.NewDecoder(dataFile).Decode(data)
	if err != nil {
		logger.Error().Err(err).Msg("[InFile.Read] cache decode failed")
		return false
	}

	logger.Debug().Msg("[InFile.Read] cache read success")

	return true
}

func (h *InFile) Write(name string, data interface{}) error {
	h.mx.Lock()
	defer h.mx.Unlock()

	// logger with struct data
	logger := h.getLogger(name)

	// filepath containing cached data
	path := h.getFilePath(name)

	// os.O_APPEND|os.O_CREATE|os.O_WRONLY
	dataFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	defer dataFile.Close()

	if err != nil {
		logger.Error().Err(err).Msg("[InFile.Write] cache write fail")
		return err
	}

	encoder := gob.NewEncoder(dataFile)
	if err = encoder.Encode(data); err != nil {
		logger.Error().Err(err).Msg("[InFile.Write] cache encode fail")
	}

	logger.Debug().Msg("[InFile.Write] cache written")

	return err
}

func (h *InFile) getFilePath(name string) string {
	return fmt.Sprintf("%s/aws.%s.gob", h.cacheDir, name)
}

func (h *InFile) isExpired(name string) bool {
	path := h.getFilePath(name)

	// get last modified time
	statFile, err := os.Stat(path)
	if err != nil {
		logger := h.getLogger(name)
		logger.Trace().Err(err).Msg("[InFile.IsExpired] expired check failed")
		return true
	}

	return time.Now().Sub(statFile.ModTime()) > h.ttl
}

func (h *InFile) getLogger(name string) zerolog.Logger {
	return log.With().
		Str("key", name).
		Str("file", h.getFilePath(name)).
		Str("store", CacheTypeFile).
		Logger()
}
