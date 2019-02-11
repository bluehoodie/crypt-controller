package store

import (
	"github.com/pkg/errors"

	"k8s.io/api/core/v1"
)

var (
	NotFoundError    = errors.New("key not found")
	InvalidDataError = errors.New("value could not be decoded into a store object")
)

type Store interface {
	Get(key string) (*Object, error)
}

type Object struct {
	Name       string            `json:"name"`
	SecretType string            `json:"secret_type,omitempty"`
	Data       map[string][]byte `json:"data"`
}

func (o *Object) GetName() string {
	return o.Name
}

func (o *Object) GetSecretType() string {
	if o.SecretType == "" {
		return string(v1.SecretTypeOpaque)
	}
	return o.SecretType
}

func (o *Object) GetData() map[string][]byte {
	if o.Data == nil {
		o.Data = make(map[string][]byte)
	}

	return o.Data
}
