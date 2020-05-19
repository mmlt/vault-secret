Vault-secret is a Mutating Admission Controller that populates core v1 Secret data with values read from HashiCorp Vault.

BEWARE Only use this for applications that consume core v1 Secret without being able to mount the secret as a volume
like for example cert-manager, imagePullSecret.

Other applications should use [Vault Agent](https://www.hashicorp.com/blog/injecting-vault-secrets-into-kubernetes-pods-via-a-sidecar/)

## Usage

### Run vaultsecret controller

Running the controller is like any admission controller.

See `--help` for configuration flags.


### Configure Vault

The source is your friend, `controllers/vault_test.go testConfigureVault()` shows how to configure Vault
kubeauth, kubenetes auth role, policy and secrets.

TODO insert vault cli commands for example here


### Create a Secret

Create a Secret with annotations;
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  annotations:
    vault.mmlt.nl/inject: "true"
    vault.mmlt.nl/inject-path: "secret/data/ns/default/example"
    vault.mmlt.nl/inject-fields: "user=name,pw=password"
```

Upon creation the Secret `data` fields will be populated with values from Vault.


## Background
 
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

At `Login` the identity of the webhook controller Pod and the namespace of the target Secret are used to select a Vault role. 
The blue lines in [flow](flow-smaller.png) show how the role name is constructed.

Vault uses TokenReview to check the Login and on success returns a token.

At `Get` a path is used to read a secret from Vault, ofcourse the path has to be allowed by a policy.
The purple lines in [flow](flow-smaller.png) show the relations between `vault.mmlt.nl/inject-path`, policy and vault secret path. 

When the values are successfully read from Vault what's left is a simple key rename (orange lines in [flow](flow-smaller.png))
and update of the Secret data field before the mutated Secret is returned to the API Server.


## Developing
 
Prerequisite:
- [kubebuilder](https://kubebuilder.io) for code generation and testenv binaries.


## Future
- Consider reconciling Secrets with Vault
- Consider changing the vault.mmlt.nl/inject="true" annotation to support other vaults.