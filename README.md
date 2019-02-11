# Crypt Controller

`crypt-controller` is a custom Kubernetes controller which handles a custom resource called a `crypt`.  A `crypt`'s spec outlines a list of keys (representing keys to fetch from a key-value store), and a list of target namespaces.  The controller will fetch values from the key-value store and transform those into Kubernetes secrets on the target namespaces.

The controller ensures that all available secrets associated with Crypt objects are present on all associated namespaces.

## Goal

The goal of this controller is to help automatically provision namespaces with secrets
 
## Installing the Crypt Custom Resource Definition

Before the controller can run correctly, the Crypt CRD must be installed on the target cluster. 

```console
$ kubectl create -f artifacts/crd.yaml
```

## Installing crypt-controller using Helm

coming soon

## Installing crypt-controller using kubectl

In order to run crypt-controller in a Kubernetes cluster quickly, the easiest way is for you to create a ConfigMap to hold crypt-controller configuration.

An example is provided at [`crypt-controller-configmap.yaml`](https://github.com/bluehoodie/crypt-controller/blob/master/example/crypt-controller-configmap.yaml)

Create k8s configmap:

```console
$ kubectl create -f examples/crypt-controller-configmap.yaml
```

Create the [Pod](https://github.com/bluehoodie/crypt-controller/blob/master/example/crypt-controller.yaml) directly, or create your own deployment:

```console
$ kubectl create -f examples/crypt-controller.yaml
```

Once the Pod is running, you could begin adding Crypt objects.  For example:

```yaml
apiVersion: core.bluehoodie.io/v1alpha1
kind: Crypt
metadata:
  name: test-crypt
spec:
  keys:
    - test/foo
  namespaces:
    - default
    - foo-*
```

This example would create a secret based on data provided in the value at `test/foo` in the default namespace and all namespaces matching the regex `foo-*`

## Data Model

The values stored in the key-values stores are expected to be JSON objects, with the following structure:

```go
type Object struct {
	Name       string            `json:"name"`
	SecretType string            `json:"secret_type,omitempty"`
	Data       map[string][]byte `json:"data"`
}
```

Secret type, if not defined, defaults to `Opaque`.

## Currently supported backends:

- Consul

## Environment variables

Required environment variables by `crypt-controller`:
* `STORE_TYPE` - ex: "consul"

In addition, you are required to provide all environment variables required to connect to the key-value storage.

## Further Development

Current TODOs include:

- Improve test coverage.
- Support Vault 
- Create Helm chart

## Contributing

Issues and pull requests welcome.