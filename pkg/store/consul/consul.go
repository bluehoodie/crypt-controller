package consul

import (
	"encoding/json"

	"github.com/bluehoodie/crypt-controller/pkg/store"

	"github.com/hashicorp/consul/api"
)

type Store struct {
	client *api.Client
}

func New(config *api.Config) (store.Store, error) {
	if config == nil {
		config = api.DefaultConfig()
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &Store{client: client}, nil
}

func (s *Store) Get(key string) (store.Object, error) {
	pair, _, err := s.client.KV().Get(key, nil)
	if err != nil {
		return nil, err
	}

	var obj map[string][]byte
	err = json.Unmarshal(pair.Value, &obj)
	if err != nil {
		return nil, store.InvalidDataError
	}

	return store.Object(obj), nil
}
