package factory

import (
	"strings"

	"github.com/bluehoodie/crypt-controller/pkg/store"
	"github.com/bluehoodie/crypt-controller/pkg/store/consul"
	"github.com/bluehoodie/crypt-controller/pkg/store/empty"
	"github.com/bluehoodie/crypt-controller/pkg/store/fake"
	"github.com/bluehoodie/crypt-controller/pkg/store/memory"

	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
)

const (
	EmptyStoreType  = "empty"
	FakeStoryType   = "fake"
	MemoryStoreType = "memory"

	ConsulStoreType = "consul"
)

type Factory struct {
	config string
}

func (f *Factory) Make(storeType string) (store.Store, error) {
	switch strings.TrimSpace(strings.ToLower(storeType)) {
	case EmptyStoreType:
		return empty.New()
	case MemoryStoreType:
		return memory.New()
	case FakeStoryType:
		return fake.New()
	case ConsulStoreType:
		return consul.New(api.DefaultConfig())
	default:
		return nil, errors.New("invalid store type")
	}
}

func NewStoreFactory(configFilePath string) *Factory {
	return &Factory{config: configFilePath}
}
