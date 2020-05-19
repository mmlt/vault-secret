package controllers

import (
	"fmt"
	kubeauth "github.com/hashicorp/vault-plugin-auth-kubernetes"
	kv "github.com/hashicorp/vault-plugin-secrets-kv"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
	"github.com/mmlt/testr"
	"github.com/mmlt/vault-secret/pkg/vault/hashivault"
	"github.com/stretchr/testify/assert"
	"net"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
	"time"
)

// TestVault runs test cases against an existing k8s cluster ICW Vault running in memory.
// The only reason to use an existing cluster is because kubeauth needs a tokenreview endpoint and tokenreview currently
// doesn't work in envtest.
// Prerequisites:
// - kubectl config current-context referring the right cluster.
// - a ServiceAccount (with Secret) exists.
// - no conflicting webhooks, check; kubectl get mutatingwebhookconfigurations
// Teardown:
// - kubectl delete mutatingwebhookconfigurations vaultsecret-webhookconfig
func TestVault(t *testing.T) {
	// ServiceAccount
	namespace, name := "default", "default"

	if !useExistingCluster {
		t.Fatal("")
	}

	logf.SetLogger(testr.New(t))

	// Instantiate Vault.
	// NB. a more lightweight alternative is: l, c := testVaultCore(t); defer l.Close()
	cl, c := testVaultCluster(t)
	defer cl.Cleanup()
	testConfigureVault(t, c, namespace, name)

	// Create client.
	// NB. during testing kubeauth can be skipped by using: client := hashivault.NewAlreadyLoggedIn(c)
	jwt := testGetServiceAccountToken(t, namespace, name)
	client, err := hashivault.NewOutsideCluster(c.Address(), "", true, jwt)
	assert.NoError(t, err)

	// Instantiate (webhook) manager.
	stop := make(chan struct{})
	testManager(t, client, stop)

	t.Run("should_get_data_fields_from_vault_kv", func(t *testing.T) {
		testCreateSecret(t, map[string]string{
			"vault.mmlt.nl/inject":        "true",
			"vault.mmlt.nl/inject-path":   "secret/path/to/test",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		}, nil)
		got := testGetSecret(t)
		assert.Equal(t, map[string]string{
			"een":  "first-vault-value",
			"twee": "second-vault-value",
		}, msb2mss(got.Data))
	})

	t.Run("should_get_data_fields_from_vault_kv-v2", func(t *testing.T) {
		testCreateSecret(t, map[string]string{
			"vault.mmlt.nl/inject":        "true",
			"vault.mmlt.nl/inject-path":   "kv/data/path/to/test",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		}, nil)
		got := testGetSecret(t)
		assert.Equal(t, map[string]string{
			"een":  "first-kv-value",
			"twee": "second-kv-value",
		}, msb2mss(got.Data))
	})

	// debugging:
	//fmt.Printf("export VAULT_ADDR=\"%s\"\n", c.Address())
	//fmt.Printf("export VAULT_DEV_ROOT_TOKEN_ID=\"%s\"\n", c.Token())
	//fmt.Printf("export VAULT_SKIP_VERIFY=true\n")
	//fmt.Printf("vault login $VAULT_DEV_ROOT_TOKEN_ID\n")
	//time.Sleep(time.Hour)

	// teardown manager
	close(stop)
	time.Sleep(time.Second) //TODO how to wait for manager shutdown?
}

// TestVaultExisting runs test cases against an existing k8s cluster running Vault.
// Prerequisites:
// - kubectl config current-context referring the right cluster.
// - Vault configured with kubernetes auth, role, policy, secret.
// - ServiceAccount that matches kubernetes auth config.
// Teardown:
// - kubectl delete mutatingwebhookconfigurations vaultsecret-webhookconfig
func TestVaultExisting(t *testing.T) {
	//t.Skip("requires setup of prerequisites and teardown")

	if !useExistingCluster {
		t.Fatal("This test requires connection to an external cluster")
	}

	logf.SetLogger(testr.New(t))

	// Update the following Vault and ServiceAccount parameters to match the remote cluster.
	c := testVaultClient(t, "https://10.152.183.139:8200", "s.kDnZs2zHDq6aY1ksKgJL8NCS")
	jwt := testGetServiceAccountToken(t, "cpe-system", "vaultsecret")
	client, err := hashivault.NewOutsideCluster(c.Address(), "", true, jwt)
	assert.NoError(t, err)

	// Instantiate (webhook) manager.
	stop := make(chan struct{})
	testManager(t, client, stop)

	t.Run("should_get_data_fields_from_vault", func(t *testing.T) {
		testCreateSecret(t, map[string]string{
			"vault.mmlt.nl/inject":        "true",
			"vault.mmlt.nl/inject-path":   "secret/data/ns/default/example",
			"vault.mmlt.nl/inject-fields": "user=name,pw=password",
		}, nil)
		got := testGetSecret(t)
		assert.Equal(t, map[string]string{
			"user": "superman",
			"pw":   "supersecret",
		}, msb2mss(got.Data))
	})

	// teardown manager
	close(stop)
	time.Sleep(time.Second) //TODO how to wait for manager shutdown?
}

// TestVaultCore starts a Vault core.
// Teardown with close(net.Listener).
// See TestCore with KV v2 doesn't generate metadata · Issue #8440 · hashicorp/vault
func testVaultCore(t *testing.T) (net.Listener, *vaultapi.Client) {
	t.Helper()

	// Create an in-memory, unsealed core.
	core, keyShares, rootToken := vault.TestCoreUnsealedWithConfig(t, &vault.CoreConfig{
		CredentialBackends: map[string]logical.Factory{
			"kubeauth": kubeauth.Factory,
		},
	})
	_ = keyShares

	// Start an HTTP server for the core.
	ln, addr := http.TestServer(t, core)

	client := testVaultClient(t, addr, rootToken)

	return ln, client
}

// TestVaultCluster starts a Vault cluster.
// Teardown with vault.TestCluster Cleanup().
func testVaultCluster(t *testing.T) (*vault.TestCluster, *vaultapi.Client) {
	t.Helper()

	cluster := vault.NewTestCluster(t, &vault.CoreConfig{
		CredentialBackends: map[string]logical.Factory{
			"kubeauth": kubeauth.Factory,
		},
		LogicalBackends: map[string]logical.Factory{
			"kv": kv.Factory,
		},
		//DevToken: "easy-to-remember",
	}, &vault.TestClusterOptions{
		HandlerFunc: http.Handler,
	})
	cluster.Start()

	core := cluster.Cores[0].Core
	vault.TestWaitActive(t, core)
	client := cluster.Cores[0].Client

	return cluster, client
}

// TestVaultClient returns a client that talks to a Vault server at addr, authenticated with rootToken.
func testVaultClient(t *testing.T, addr, rootToken string) *vaultapi.Client {
	t.Helper()

	conf := vaultapi.DefaultConfig()
	conf.Address = addr
	err := conf.ConfigureTLS(&vaultapi.TLSConfig{
		Insecure: true,
	})
	assert.NoError(t, err)

	client, err := vaultapi.NewClient(conf)
	assert.NoError(t, err)

	client.SetToken(rootToken)

	return client
}

// TestConfigureVault uses client to prep Vault with auth backend, roles, policies and secrets.
// The kubernetes auth role requires the Pod to run in namespace/name.
func testConfigureVault(t *testing.T, client *vaultapi.Client, namespace, name string) {
	t.Helper()

	var err error

	// Setup KV.
	// by default a kv-v1 is mounted at secret/
	// mount a kv-v2 (https://vaultproject.io/api/secret/kv/kv-v2.html) also.
	err = client.Sys().Mount("kv/", &vaultapi.MountInput{
		Type: "kv-v2",
	})
	assert.NoError(t, err)

	// Setup auth/kubernetes
	// https://www.vaultproject.io/docs/auth/kubernetes
	// https://www.vaultproject.io/api-docs/auth/kubernetes

	// Mount the auth backend.
	err = client.Sys().EnableAuthWithOptions("kubernetes", &vaultapi.EnableAuthOptions{
		Type: "kubeauth",
	})
	assert.NoError(t, err)

	// Configure the auth backend.
	_, err = client.Logical().Write("auth/kubernetes/config", map[string]interface{}{
		// Host must be a host string, a host:port pair, or a URL to the base of the Kubernetes API server.
		"kubernetes_host": cfg.Host,
		// PEM encoded CA cert for use by the TLS client used to talk with the Kubernetes API.
		// NOTE: Every line must end with a newline: \n
		"kubernetes_ca_cert": string(cfg.CAData),
	})
	assert.NoError(t, err)

	// Create a Vault role to login with.
	// Assume the 'default' service account from the 'default' namespace is pre-created.
	_, err = client.Logical().Write(fmt.Sprintf("auth/kubernetes/role/vaultsecret-%s", testNSN.Namespace), map[string]interface{}{
		"bound_service_account_names":      name,
		"bound_service_account_namespaces": namespace,
		"policies": []string{
			"default",
			"ns-default",
		},
		"ttl": "1h",
	})
	assert.NoError(t, err)
	// assert that the k8s service account used by this role exists and has a secret.
	testGetServiceAccountToken(t, namespace, name)

	// Create policies.
	err = client.Sys().PutPolicy("ns-default", `
path "secret/path/to/*" {
  capabilities = ["read"]
}
path "kv/data/path/to/*" {
  capabilities = ["read"]
}
`)
	assert.NoError(t, err)

	// Create secrets.
	// kv-v1
	_, err = client.Logical().Write("secret/path/to/test", map[string]interface{}{
		"one": "first-vault-value",
		"two": "second-vault-value",
	})
	assert.NoError(t, err)
	// kv-v2
	_, err = client.Logical().Write("kv/data/path/to/test", map[string]interface{}{
		"data": map[string]interface{}{ //TODO
			"one": "first-kv-value",
			"two": "second-kv-value",
		},
	})
	assert.NoError(t, err)
}
