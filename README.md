# Crypt Controller [![Build Status](https://travis-ci.org/bluehoodie/crypt-controller.svg?branch=master)](https://travis-ci.org/bluehoodie/crypt-controller)

## Goal

The goal of this controller is to help automatically provision namespaces with secrets, and to keep those secrets up to date, using a key-value store as a central repository of data.

This is done using a Custom Resource Definition called a `Crypt`, which defines a list of target namespaces and a list of secrets sources.  The controller takes care of fetching the secret data from the source and making sure all the secrets are present and up to date.

## Installing crypt-controller using Helm

```console
$  helm install ./chart --set {{custom values}}
```

The `values.yaml` file in the chart folder requires some configuration in order to run correctly - most notably the store configuration section:

```yaml
storeType: invalid
store:
  consul:
    enabled: false
    env: {}
  vault:
    enabled: false
    env: {}
```

The `storeType` must be set to a valid storeType (either consul or vault), the corresponding node in the store section must be set to `enabled: true` and all required environment variables must be set in its `env` section.

## Data Model

The values stored in the key-values store are expected to be key value maps of type string -> []byte (ie: a simple json with string keys and base64-encoded values.)

## Usage

Once the controller is running on your cluster, you can create crypt resources as you would create any other resource. An example crypt resource definition:

```yaml
apiVersion: core.bluehoodie.io/v1alpha1
kind: Crypt
metadata:
  name: test-crypt
  namespace: default
spec:
  secrets:
    - name: foo
      key: crypt/dev/foo
    - name: bar
      key: crypt/dev/bar
  namespaces:
    - dev-*
```

This crypt will automatically pull data from keys `crypt/dev/foo` and `crypt/dev/bar` and create secrets with names `foo` and `bar`, respectively, in all namespaces matching the pattern `dev-*`.

Expected behaviour:
- If the secrets managed by a crypt are deleted, then the controller will re-create them.
- If new namespaces appear, then crypts will be checked to see if any secrets need to be created in this namespace.
- If the data in the store changes, then the data in the secrets will be updated (after a small resync period delay).
- If the crypt resource is deleted, all of its associated secrets are also deleted.

## Contributing

Issues and pull requests welcome.