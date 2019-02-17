package store

import (
	"github.com/pkg/errors"
)

var (
	NotFoundError    = errors.New("key not found")
	InvalidDataError = errors.New("value could not be decoded into a store object")
)

type Store interface {
	Get(key string) (Object, error)
}

type Object map[string][]byte

func (o Object) GetData() map[string][]byte {
	if o == nil {
		data := make(map[string][]byte)
		return data
	}

	return map[string][]byte(o)
}
