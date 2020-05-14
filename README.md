Vault-secret is a Mutating Admission Controller that populates core v1 Secret data with values read from HashiCorp Vault.

BEWARE Only use this for applications that consume core v1 Secret without being able to mount the secret as a volume
like for example cert-manager, imagePullSecret.

Other applications should use [Vault Agent](https://www.hashicorp.com/blog/injecting-vault-secrets-into-kubernetes-pods-via-a-sidecar/)

## Usage

See `--help`

### Installing the controller


### Setup Vault

See `controllers/vault_test.go testVault()`


### Create a Secret
To get the data fields of a Secret populated it has to be annotated;
```
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  annotations:
    vault.mmlt.nl/inject="true"
	vault.mmlt.nl/inject-path="secret/path/to/secret"
	vault.mmlt.nl/inject-fields="user=name,pw=password"
data:
```

## Developing
Prerequisite: [kubebuilder](https://kubebuilder.io) for code generation and testenv binaries.

E2E tests:
- TestFakevault
- TestVault with root token
- TestVault with kubeauth won't work in envtest (because of TokenReview endpoint not working) 

All E2E tests run in microk8s (set useExternalCluster = true).


## Background


When getting a secret from Vault the Pod identity of the webhook controller and the namespace of the target Secret are to
select the Vault Policy.

For example the following Vault role applies the Vault policy `policy-that-resticts-access-to-mynamespace` 
when the vaultsecret webhook controller Pod named `controller-name` running in namespace `controller-namespace`
receives a request for a Secret that is created in `mynamespace`
```
vault write auth/kubernetes/role/vaultsecret-mynamespace
  bound_service_account_name = controller-name
  bound_service_account_namespace = controller-namespace
  policies: policy-that-resticts-access-to-mynamespace
```

The `policy-that-resticts-access-to-mynamespace` has to be created like this:
TODO

And of course the secrets have to be created:
TODO
 
 
The sequence of events up-on creation of a Secret with annotations looks like this: 
```
+---+             +-----+             +-------------+                              +-------+
| X |             | API |             | Controller  |                              | Vault |
+---+             +-----+             +-------------+                              +-------+
  |                  |                       |                                         |
  | Create Secret    |                       |                                         |
  |----------------->|                       |                                         |
  |                  |                       |                                         |
  |                  | Admission req.        |                                         |
  |                  |---------------------->|                                         |
  |                  |                       |                                         |
  |                  |                       | Login (role incl. Secret namespace)     |
  |                  |                       |---------------------------------------->|
  |                  |                       |                                         |
  |                  |                       |                             TokenReview |
  |                  |<----------------------------------------------------------------|
  |                  |                       |                                         |
  |                  |                       |                             vault token |
  |                  |                       |<----------------------------------------|
  |                  |                       |                                         |
  |                  |                       | Get                                     |
  |                  |                       |---------------------------------------->|
  |                  |                       |                                         |
  |                  |        mutated Secret |                                         |
  |                  |<----------------------|                                         |
  |                  |                       |                                         |
```

Rendered with https://textart.io/sequence
```
object X API Controller Vault API
X->API: Create Secret
API->Controller: Admission req.
Controller->Vault: Login (role incl. Secret namespace)
Vault->API: TokenReview
Vault->Controller: vault token
Controller->Vault: Get
Controller->API: mutated Secret
```

## TODO
- Consider reconciling Secrets with Vault
- Consider changing the vault.mmlt.nl/inject="true" annotation to support other vaults.