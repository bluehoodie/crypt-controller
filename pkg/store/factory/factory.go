package factory

import (
	"strings"

	"github.com/bluehoodie/crypt-controller/pkg/store"
	"github.com/bluehoodie/crypt-controller/pkg/store/consul"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

const (
	ConsulStoreType = "consul"
)

type Factory struct {
	config string
}

func (f *Factory) Make(storeType string) (store.Store, error) {
	switch strings.TrimSpace(strings.ToLower(storeType)) {
	case ConsulStoreType:
		return consul.New(consulapi.DefaultConfig())
	default:
		return nil, errors.New("invalid store type")
	}
}

func NewStoreFactory(configFilePath string) *Factory {
	return &Factory{config: configFilePath}
}
