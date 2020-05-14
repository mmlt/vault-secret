package mutator

import (
	"context"
	"encoding/json"
	"github.com/mmlt/vault-secret/pkg/vault"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-v1-secret,mutating=true,failurePolicy=fail,groups="",resources=secrets,verbs=create;update,versions=v1,name=msecret.kb.io

// SecretMutator populates Secret data with value(s) read from Vault.
type SecretMutator struct {
	// VaultAuthPath is the mount path of the kubeauth backend (typically "kubernetes")
	VaultAuthPath string

	// VaultRole is a template that results in a role name.
	// Arguments: {ns} for namespace, {n} for name.
	// For example "vaultsecret-{ns}" produces "vaultsecret-default" when the Secret is in namespace "default".
	VaultRole string

	// VaultSecretPath is template that results in a Vault path.
	// Arguments: {ns} for namespace, {n} for name, {p} for the vault.mmlt.nl/inject-path annotation value.
	// Example: "secret/{ns}/{p}"
	VaultSecretPath string

	// Vault accessor.
	Vault vault.Loginer

	// Decoder for incoming k8s objects.
	decoder *admission.Decoder
}

// Handle a admission request.
// Read annotations, query Vault, set Secret data.
func (m *SecretMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	secret := &corev1.Secret{}

	err := m.decoder.Decode(req, secret)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if secret.Annotations == nil {
		// not annotated, do not process this secret.
		return admission.Allowed("")
	}

	// Enable the injection of data fields. This should be set to a true or false value. Defaults to false.
	enabled := secret.Annotations["vault.mmlt.nl/inject"]
	// The path in Vault where the secret is located relative to VaultSecretPath.
	path := secret.Annotations["vault.mmlt.nl/inject-path"]
	// A comma separated list of k8s secret field name = vault secret field name pairs.
	fields := secret.Annotations["vault.mmlt.nl/inject-fields"]

	if enabled != "true" || path == "" || fields == "" {
		// not properly annotated, do not process this secret.
		return admission.Allowed("")
	}

	c, err := m.Vault.Login(m.VaultAuthPath, replaceNSN(m.VaultRole, secret.Namespace, secret.Name))
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	_ = ctx // use in Get() when github.com/hashicorp/vault/api.Read() supports context.
	data, err := c.Get(replaceNSNP(m.VaultSecretPath, secret.Namespace, secret.Name, path))
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, p := range strings.Split(fields, ",") {
		v := strings.Split(p, "=")
		if len(v) != 2 {
			continue
		}
		secret.Data[v[0]] = []byte(data[v[1]])
	}

	js, err := json.Marshal(secret)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, js)
}

// InjectDecoder implements the DecoderInjector interface.
func (m *SecretMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

// ReplaceNSNP replaces {ns} with namespace, {n} with name and {p} with path and returns the result.
func replaceNSNP(in, namespace, name, path string) string {
	s := strings.ReplaceAll(in, "{p}", path)
	return replaceNSN(s, namespace, name)
}

// ReplaceNSN replaces {ns} with namespace and {n} with name and returns the result.
func replaceNSN(in, namespace, name string) string {
	s := strings.ReplaceAll(in, "{ns}", namespace)
	return strings.ReplaceAll(s, "{n}", name)
}
