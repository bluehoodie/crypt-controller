package memory

import "github.com/bluehoodie/crypt-controller/pkg/store"

type Store struct {
	m map[string]store.Object
}

func New(m map[string]store.Object) (store.Store, error) {
	s := Store{
		m: m,
	}

	return &s, nil
}

func (s *Store) Get(key string) (store.Object, error) {
	v, ok := s.m[key]
	if !ok {
		return nil, store.NotFoundError
	}
	return v, nil
}
