package vault

import (
	"fmt"

	"github.com/bluehoodie/crypt-controller/pkg/store"

	"github.com/hashicorp/vault/api"
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
	secret, err := s.client.Logical().Read(fmt.Sprintf("secret/data/%s", key))
	if err != nil {
		return nil, err
	}

	if secret == nil {
		return nil, store.NotFoundError
	}

	obj, err := dataToObject(secret.Data)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func dataToObject(data map[string]interface{}) (store.Object, error) {
	o := store.Object(make(map[string][]byte))

	for key, value := range data {
		bytes, ok := value.([]byte)
		if !ok {
			return nil, store.InvalidDataError
		}
		o[key] = bytes
	}

	return o, nil
}
