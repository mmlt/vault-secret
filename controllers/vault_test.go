package controllers

import (
	kubeauth "github.com/hashicorp/vault-plugin-auth-kubernetes"
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

func TestVault(t *testing.T) {
	stop := make(chan struct{})

	logf.SetLogger(testr.New(t))

	// Instantiate Vault.
	l, c := testVault(t)
	defer l.Close()

	//client := hashivault.NewAlreadyLoggedIn(c)
	jwt := testGetServiceAccountToken(t, "default", "default")
	client, err := hashivault.NewOutsideCluster(c.Address(), "", true, jwt)
	assert.NoError(t, err)

	// Instantiate (webhook) manager.
	testManager(t, client, stop)

	t.Run("should_get_data_fields_from_vault", func(t *testing.T) {
		testCreateSecret(t, map[string]string{
			"vault.mmlt.nl/inject":        "true",
			"vault.mmlt.nl/inject-path":   "secret/path/to/test",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		})
		got := testGetSecret(t)
		assert.Equal(t, 3, len(got.Data))
		assert.Equal(t, map[string]string{
			"een":             "first-vault-value",
			"twee":            "second-vault-value",
			"shouldNotChange": "value",
		}, msb2mss(got.Data))

		// debugging:
		//fmt.Printf("export VAULT_ADDR=\"%s\"\n", c.Address())
		//fmt.Printf("export VAULT_DEV_ROOT_TOKEN_ID=\"%s\"\n", c.Token())
		//time.Sleep(time.Hour)
	})

	// teardown manager
	close(stop)
	time.Sleep(time.Second) //TODO how to wait for manager shutdown?
}

// TestVault starts a Vault instance and preps it with roles and secrets.
func testVault(t *testing.T) (net.Listener, *vaultapi.Client) {
	t.Helper()

	var err error
	// Create an in-memory, unsealed core.
	// Enable kubeauth backend.
	core, keyShares, rootToken := vault.TestCoreUnsealedWithConfig(t, &vault.CoreConfig{
		CredentialBackends: map[string]logical.Factory{
			"kubeauth": kubeauth.Factory,
		},
	})
	_ = keyShares

	// Start an HTTP server for the core.
	ln, addr := http.TestServer(t, core)

	// Create a client that talks to the server, initially authenticating with the root token.
	conf := vaultapi.DefaultConfig()
	conf.Address = addr

	client, err := vaultapi.NewClient(conf)
	if err != nil {
		t.Fatal(err)
	}
	client.SetToken(rootToken)

	// Setup auth/kubernetes
	// https://www.vaultproject.io/docs/auth/kubernetes
	// https://www.vaultproject.io/api-docs/auth/kubernetes

	// Mount the auth backend.
	err = client.Sys().EnableAuthWithOptions("kubernetes", &vaultapi.EnableAuthOptions{
		Type: "kubeauth",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Configure the auth backend.
	_, err = client.Logical().Write("auth/kubernetes/config", map[string]interface{}{
		// Host must be a host string, a host:port pair, or a URL to the base of the Kubernetes API server.
		"kubernetes_host": cfg.Host,
		// PEM encoded CA cert for use by the TLS client used to talk with the Kubernetes API.
		// NOTE: Every line must end with a newline: \n
		"kubernetes_ca_cert": string(cfg.CAData),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a Vault role to login with.
	// Assume the 'default' service account from the 'default' namespace is pre-created.
	_, err = client.Logical().Write("auth/kubernetes/role/vaultsecret", map[string]interface{}{
		"bound_service_account_names":      "default",
		"bound_service_account_namespaces": "default",
		"policies": []string{
			"default",
			"ns-default",
		},
		"ttl": "1h", //TODO can be short as we login for each request
	})
	if err != nil {
		t.Fatal(err)
	}
	// assert the service account exists and has a secret.
	testGetServiceAccountToken(t, "default", "default")

	// Create test secrets and policies in Vault.

	err = client.Sys().PutPolicy("ns-default", `
path "secret/path/to/*" {
  capabilities = ["read"]
}
`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Logical().Write("secret/path/to/test", map[string]interface{}{
		"one": "first-vault-value",
		"two": "second-vault-value",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Logical().Write("secret/path/to/numbers", map[string]interface{}{
		"integer": "3",
		"float":   "3.14",
	})
	if err != nil {
		t.Fatal(err)
	}

	return ln, client
}
