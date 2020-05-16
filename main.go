/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"github.com/mmlt/vault-secret/controllers"
	"github.com/mmlt/vault-secret/pkg/mutator"
	"github.com/mmlt/vault-secret/pkg/vault/hashivault"
	"io/ioutil"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	//_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	setupLog = ctrl.Log.WithName("setup")

	usage = `%[1]s %[2]s
%[1]s is a Mutating Admission Controller that populates core v1 Secret data with values read from HashiCorp Vault.

Secret annotations:
  vault.mmlt.nl/inject="true" - Enable the injection of data fields. This should be set to a true or false value. Defaults to false.
  vault.mmlt.nl/inject-path="path/to/secret" - The path in Vault where the secret is located relative to vault-secret-path.
  vault.mmlt.nl/inject-fields="user=name,pw=password" - A comma separated list of k8s secret field name = vault secret field name pairs.

Commandline flags:
`
	// Version is set during build.
	Version string
)

func main() {
	vaultURL := flag.String("vault-url", "https://vault.example.com",
		"The URL of the Vault server")
	vaultCAFile := flag.String("vault-ca-file", "",
		"The path of the Vault server CA")
	vaultTLSInsecure := flag.Bool("vault-tls-insecure", false,
		"Allow insecure TLS connections")
	vaultAuthPath := flag.String("vault-auth-path", "kubernetes",
		"The path of the Vault kubeauth credential backend mount")
	vaultRole := flag.String("vault-role", "vaultsecret",
		"The template that results in a role name. Arguments: {ns} for namespace, {n} for name. \n"+
			"for example \"vaultsecret-{ns}\" produces \"vaultsecret-default\" when the Secret is in namespace \"default\"")
	vaultSecretPath := flag.String("vault-secret-path", "secret/{ns}/{p}",
		"The template that results in a Vault path.\n"+
			"Arguments: {ns} for namespace, {n} for name, {p} for the vault.mmlt.nl/inject-path annotation value")
	metricsAddr := flag.String("metrics-addr", ":8080",
		"The address the metric endpoint binds to.")
	webhookCertDir := flag.String("webhook-cert-dir", "/var/run/webhook",
		"The directory containing the webhook server tls.key and tls.crt files.")
	webhookPort := flag.Int("webhook-port", 9443,
		"The port the webhook server binds to.")

	//enableLeaderElection := flag.Bool("enable-leader-election", false,
	//	"Enable leader election for controller manager. "+
	//		"Enabling this will ensure there is only one active controller manager.")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, usage, filepath.Base(os.Args[0]), Version)
		flag.PrintDefaults()
	}
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	ctrl.Log.Info("starting", "version", Version)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		//Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		Port:               *webhookPort,
		CertDir:            *webhookCertDir,
		//LeaderElection:     enableLeaderElection,
		//LeaderElectionID:   "c87fed36.mmlt.nl",
	})
	exitWhenError("creating manager", err)

	var vaultCA []byte
	if *vaultCAFile != "" {
		vaultCA, err = ioutil.ReadFile(*vaultCAFile)
		exitWhenError("reading vault-ca-file", err)
	}

	client, err := hashivault.New(*vaultURL, string(vaultCA), *vaultTLSInsecure)
	exitWhenError("creating Vault client", err)

	hookServer := mgr.GetWebhookServer()
	hookServer.Register(controllers.WebhookPath, &webhook.Admission{
		Handler: &mutator.SecretMutator{
			Vault:           client,
			VaultAuthPath:   *vaultAuthPath,
			VaultRole:       *vaultRole,
			VaultSecretPath: *vaultSecretPath,
			Log:             ctrl.Log,
		},
	})

	setupLog.Info("starting manager")
	err = mgr.Start(ctrl.SetupSignalHandler())
	exitWhenError("start manager", err)
}

func exitWhenError(msg string, err error) {
	if err != nil {
		setupLog.Error(err, msg)
		os.Exit(1)
	}
}
