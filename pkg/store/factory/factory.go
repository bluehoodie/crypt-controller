package factory

import (
	"strings"

	"github.com/bluehoodie/crypt-controller/pkg/store"
	"github.com/bluehoodie/crypt-controller/pkg/store/consul"
	"github.com/bluehoodie/crypt-controller/pkg/store/vault"

	consulapi "github.com/hashicorp/consul/api"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

const (
	ConsulStoreType = "consul"
	VaultStoreType  = "vault"
)

type Factory struct {
	config string
}

func (f *Factory) Make(storeType string) (store.Store, error) {
	switch strings.TrimSpace(strings.ToLower(storeType)) {
	case ConsulStoreType:
		return consul.New(consulapi.DefaultConfig())
	case VaultStoreType:
		return vault.New(vaultapi.DefaultConfig())
	default:
		return nil, errors.New("invalid store type")
	}
}

func NewStoreFactory(configFilePath string) *Factory {
	return &Factory{config: configFilePath}
}
