package empty

import "github.com/bluehoodie/crypt-controller/pkg/store"

type Store struct {
}

func New() (store.Store, error) {
	return &Store{}, nil
}

func (*Store) Get(key string) (store.Object, error) {
	return nil, store.NotFoundError
}
