package mutator

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-v1-secret,mutating=true,failurePolicy=fail,groups="",resources=secrets,verbs=create;update,versions=v1,name=msecret.kb.io

// SecretMutator populates Secret data with value(s) read from Vault.
type SecretMutator struct {
	Client  client.Client
	decoder *admission.Decoder

	Vault VaultGetter
}

type VaultGetter interface {
	// Get values from vault.
	Get(namespace, name, path string) (map[string][]byte, error)
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
		return admission.Allowed("") //TODO it is not for us to handle
	}

	// Enable the injection of data fields. This should be set to a true or false value. Defaults to false.
	enabled := secret.Annotations["vault.mmlt.nl/inject"]
	// The path in Vault where the secret is located.
	path := secret.Annotations["vault.mmlt.nl/inject-path"]
	// A comma separated list of k8s secret field name = vault secret field name pairs.
	fields := secret.Annotations["vault.mmlt.nl/inject-fields"]

	if enabled != "true" || path == "" || fields == "" {
		return admission.Allowed("") //TODO it is not for us to handle
	}

	data, err := m.Vault.Get(secret.Namespace, secret.Name, path)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	for _, p := range strings.Split(fields, ",") {
		v := strings.Split(p, "=")
		if len(v) != 2 {
			continue
		}
		secret.Data[v[0]] = data[v[1]]
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
