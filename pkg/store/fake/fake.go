package fake

import "github.com/bluehoodie/crypt-controller/pkg/store"

type Store struct {
}

func New() (store.Store, error) {
	return &Store{}, nil
}

func (s *Store) Get(key string) (*store.Object, error) {
	data := make(map[string][]byte)
	res := store.Object{
		Name: "fake",
		Data: data,
	}
	return &res, nil
}
