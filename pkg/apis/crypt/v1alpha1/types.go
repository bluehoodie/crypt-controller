/*
Copyright 2017 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Crypt is a specification for a Crypt resource
type Crypt struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CryptSpec   `json:"spec"`
	Status CryptStatus `json:"status"`
}

type CryptSpec struct {
	Secrets    []SecretDefinition `json:"secrets"`
	Namespaces []string           `json:"namespaces"`
}

type SecretDefinition struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Key  string `json:"key"`
}

func (in *SecretDefinition) GetName() string {
	return in.Name
}

func (in *SecretDefinition) GetType() string {
	if in.Type == "" {
		return string(v1.SecretTypeOpaque)
	}
	return in.Type
}

func (in *SecretDefinition) GetKey() string {
	return in.Key
}

type CryptStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CryptList is a list of Crypt resources
type CryptList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Crypt `json:"items"`
}
