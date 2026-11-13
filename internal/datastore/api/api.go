package api

import (
	"sync"

	"github.com/padok-team/burrito/internal/burrito/config"
	"github.com/padok-team/burrito/internal/datastore/storage"
)

type API struct {
	config  *config.Config
	Storage storage.Storage
	lockMu  sync.Mutex
	locks   map[string][]byte
}

func New(c *config.Config) *API {
	return &API{
		config: c,
		locks:  make(map[string][]byte),
	}
}
