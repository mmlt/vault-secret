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

package controllers

import (
	"github.com/mmlt/vault-secret/pkg/mutator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"testing"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	const webhookPath = "/mutate-v1-secret"
	var webhookInstallOptions = testWebhookInstallOptions(webhookPath)

	By("setting up the test environment")
	testEnv = &envtest.Environment{
		WebhookInstallOptions: webhookInstallOptions,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Setup manager (similar to main.go)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		//Scheme:             scheme,
		//MetricsBindAddress: metricsAddr,
		Host:           testEnv.WebhookInstallOptions.LocalServingHost,
		Port:           testEnv.WebhookInstallOptions.LocalServingPort,
		CertDir:        testEnv.WebhookInstallOptions.LocalServingCertDir,
		LeaderElection: false,
	})
	Expect(err).NotTo(HaveOccurred())

	// Setup webhook.
	hookServer := mgr.GetWebhookServer()
	hookServer.Register(webhookPath, &webhook.Admission{
		Handler: &mutator.SecretMutator{
			Client: mgr.GetClient(),
			Vault: testVault(map[string][]byte{
				"one": []byte("first-value"),
				"two": []byte("second-value"),
			}),
		},
	})

	// Start manager.
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).NotTo(HaveOccurred())
	}()

	By("waiting for webhook to be serving")
	envtest.WaitForWebhooks(mgr.GetConfig(), webhookInstallOptions.MutatingWebhooks, webhookInstallOptions.ValidatingWebhooks, webhookInstallOptions)

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func testWebhookInstallOptions(webhookPath string) envtest.WebhookInstallOptions {
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

type testVault map[string][]byte

func (v testVault) Get(namespace, name, path string) (map[string][]byte, error) {
	return v, nil
}
