package controllers

import (
	"context"
	"fmt"
	"github.com/mmlt/vault-secret/pkg/mutator"
	"github.com/mmlt/vault-secret/pkg/vault"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"testing"
)

// TestMain instantiates the following vars for usage in tests.
var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
)

// Tests use the following config.
var (
	// When true the kube/config current context cluster will be used.
	// When false the envtest apiserver will be used (NB. envtest currently doesn't support tokenreview)
	useExistingCluster = true
	// Namespace and name for test resources.
	testNSN = types.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}

	testCtx = context.Background()
)

func TestMain(m *testing.M) {
	// Setup.

	//logf.SetLogger(zap.LoggerTo(GinkgoWriter, true)) TODO

	testEnv = &envtest.Environment{
		UseExistingCluster:    &useExistingCluster,
		WebhookInstallOptions: webhookInstallOptions(WebhookPath),
		//AttachControlPlaneOutput: true,
		//KubeAPIServerFlags:    append(envtest.DefaultKubeAPIServerFlags, ),
	}

	var err error
	cfg, err = testEnv.Start()
	mustNotErr("starting testenv", err)

	err = corev1.AddToScheme(scheme.Scheme)
	mustNotErr("adding schema", err)

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	mustNotErr("creating client", err)

	fmt.Println(cfg.Host)

	// Run.
	r := m.Run()

	// Teardown.
	err = testEnv.Stop()
	mustNotErr("stopping testenv", err)

	os.Exit(r)
}

// TestStartManager starts a Manager with the provided vault.
func testManager(t *testing.T, vault vault.Loginer, stop <-chan struct{}) {
	t.Helper()

	// Setup manager (similar to main.go)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		//Scheme:             scheme,
		//MetricsBindAddress: metricsAddr,
		Host:           testEnv.WebhookInstallOptions.LocalServingHost,
		Port:           testEnv.WebhookInstallOptions.LocalServingPort,
		CertDir:        testEnv.WebhookInstallOptions.LocalServingCertDir,
		LeaderElection: false,
	})
	assert.NoError(t, err)

	// Setup webhook handler.
	hookServer := mgr.GetWebhookServer()
	hookServer.Register(WebhookPath, &webhook.Admission{
		Handler: &mutator.SecretMutator{
			Vault:           vault,
			VaultAuthPath:   "kubernetes",
			VaultRole:       "vaultsecret-{ns}",
			VaultSecretPath: "{p}",
			Log:             logf.Log,
		},
	})

	// Start manager.
	go func() {
		err = mgr.Start(stop)
		mustNotErr("starting manager", err)
	}()

	t.Log("waiting for webhook to be serving")
	o := webhookInstallOptions(WebhookPath)
	err = envtest.WaitForWebhooks(mgr.GetConfig(), o.MutatingWebhooks, o.ValidatingWebhooks, o)
	assert.NoError(t, err)
}

// WebhookInstallOptions returns the options to configure a test environment.
func webhookInstallOptions(webhookPath string) envtest.WebhookInstallOptions {
	failPolicy := admissionregistrationv1.Fail

	return envtest.WebhookInstallOptions{
		MutatingWebhooks: []runtime.Object{
			&admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vaultsecret-webhookconfig",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MutatingWebhookConfiguration",
					APIVersion: "admissionregistration.k8s.io/v1beta1",
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name:          "vaultsecret.mmlt.nl",
						FailurePolicy: &failPolicy,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Path: &webhookPath,
							},
						},
						Rules: []admissionregistrationv1.RuleWithOperations{
							{
								Operations: []admissionregistrationv1.OperationType{
									admissionregistrationv1.Create,
								},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"secrets"},
								},
							},
						},
					},
				},
			},
		},
	}
}
