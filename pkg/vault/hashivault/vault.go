package hashivault

import (
	"fmt"
	"github.com/hashicorp/vault/api"
	"github.com/mmlt/vault-secret/pkg/vault"
	"io/ioutil"
)

// New returns a config to access Vault with kubernetes authentication.
// Expect to be running in-cluster.
// - url is the URL of the Vault server.
// - ca is the CA of the Vault server.
// - insecure true disables TLS checks.
func New(url, ca string, insecure bool) (vault.Loginer, error) {
	return NewOutsideCluster(url, ca, insecure, "")
}

// New returns a config to access Vault with kubernetes authentication.
// Expect to be running out-of-cluster.
func NewOutsideCluster(url, ca string, insecure bool, jwt string) (vault.Loginer, error) {
	c := &config{
		config: api.DefaultConfig(),
		jwt:    jwt,
	}
	c.config.Address = url
	err := c.config.ConfigureTLS(&api.TLSConfig{
		CACert:   ca,
		Insecure: insecure,
	})

	return c, err
}

// Config to access a Vault with kubernetes authentication.
type config struct {
	// Vault config config.
	config *api.Config
	// JWT is the token to authenticate with k8s API server.
	// Only set when testing
	jwt string
}

// Login and on success set vault token in the receiver.
// AuthPath is the path of the Vault credential backend mount, typically "kubernetes"
// Role is a Vault role.
func (c *config) Login(authPath, role string) (vault.Getter, error) {
	const tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	clnt, err := api.NewClient(c.config)
	if err != nil {
		return nil, err
	}

	var jwt string
	if c.jwt == "" {
		// running as pod in cluster
		b, err := ioutil.ReadFile(tokenPath)
		if err != nil {
			return nil, err
		}
		jwt = string(b)
	} else {
		// out-of-cluster (for testing)
		jwt = c.jwt
	}

	p := fmt.Sprintf("auth/%s/login", authPath)
	d := map[string]interface{}{"jwt": jwt, "role": role}
	secret, err := clnt.Logical().Write(p, d) //TODO retry or let caller retry?
	if err != nil {
		return nil, err
	}

	clnt.SetToken(secret.Auth.ClientToken)

	return &client{
		client: clnt,
	}, nil
}

// Client to access Vault.
type client struct {
	client *api.Client
}

func (c *client) Get(path string) (map[string]string, error) {
	secret, err := c.client.Logical().Read(path)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, fmt.Errorf("path not found: %s", path)
	}

	data := secret.Data

	// handle KV version 2 data.
	if len(data) == 2 {
		_, d := data["data"]
		_, md := data["metadata"]
		if d && md {
			d, ok := data["data"].(map[string]interface{})
			if ok {
				data = d
			}
		}
	}

	r := map[string]string{}
	for k, v := range data {
		r[k] = fmt.Sprint(v)
	}
	return r, nil
}

// NewAlreadyLoggedIn returns a config to access Vault with an already authenticated client.
// Mainly for testing.
func NewAlreadyLoggedIn(client *api.Client) *loggedinClient {
	return &loggedinClient{client: client}
}

// LoggedinClient provides access to Vault with a config that is already authenticated.
type loggedinClient struct {
	client *api.Client
}

func (c *loggedinClient) Login(_, _ string) (vault.Getter, error) {
	return &client{client: c.client}, nil
}

var _ vault.Loginer = &loggedinClient{}
