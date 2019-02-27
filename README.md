# Crypt Controller

## Goal

The goal of this controller is to help automatically provision namespaces with secrets, and to keep those secrets up to date, using a key-value store as a central repository of data.

This is done using a Custom Resource Definition called a `Crypt`, which defines a list of target namespaces and a list of secrets sources.  The controller takes care of fetching the secret data from the source and making sure all the secrets are present and up to date.

## Installing the Crypt Custom Resource Definition

Before the controller can run correctly, the Crypt CRD must be installed on the target cluster. 

```console
$ kubectl create -f artifacts/crd.yaml
```

## Installing crypt-controller using Helm

coming soon

## Data Model

The values stored in the key-values stores are expected to be key value maps of type string -> []byte

Secret type, if not defined, defaults to `Opaque`.

## Currently supported backends:

- Consul
- Vault

## Environment variables

Required environment variables by `crypt-controller`:
* `STORE_TYPE` - ex: "consul" or "vault"

In addition, you are required to provide all environment variables required to connect to the key-value storage.

## Further Development

Current TODOs include:

- Improve test coverage

## Contributing

Issues and pull requests welcome.