package controllers

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("secret mutation webhook", func() {
	It("should leave the data fields as-is when the Secret is not annotated", func() {
		testCreateSecret(nil)
		got := testGetSecret()
		Expect(len(got.Data)).To(Equal(1))
	})

	It("should set data fields when the Secret is properly annotated", func() {
		testCreateSecret(map[string]string{
			"vault.mmlt.nl/inject":        "true",
			"vault.mmlt.nl/inject-path":   "path/to/secret",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		})
		got := testGetSecret()
		Expect(len(got.Data)).To(Equal(3))
		Expect(got.Data).To(HaveKeyWithValue("een", []byte("first-value")))
		Expect(got.Data).To(HaveKeyWithValue("twee", []byte("second-value")))
		Expect(got.Data).To(HaveKeyWithValue("shouldNotChange", []byte("value")))
	})

	It("should leave the data fields as-is when 'inject' is false", func() {
		testCreateSecret(map[string]string{
			"vault.mmlt.nl/inject":        "false",
			"vault.mmlt.nl/inject-path":   "path/to/secret",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		})
		got := testGetSecret()
		Expect(len(got.Data)).To(Equal(1))
	})

	It("should leave the data fields as-is when 'inject' is missing", func() {
		testCreateSecret(map[string]string{
			"vault.mmlt.nl/inject-path":   "path/to/secret",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		})
		got := testGetSecret()
		Expect(len(got.Data)).To(Equal(1))
	})

	AfterEach(func() {
		obj := &corev1.Secret{}
		err := k8sClient.Get(testCtx, testNSN, obj)
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Delete(testCtx, obj)
		Expect(err).NotTo(HaveOccurred())
	})
})

var (
	testNSN = types.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}
	testCtx = context.Background()
)

func testCreateSecret(annotations map[string]string) {
	secret := &corev1.Secret{}
	secret.Namespace = testNSN.Namespace
	secret.Name = testNSN.Name
	secret.Data = map[string][]byte{
		"shouldNotChange": []byte("value"),
	}
	secret.Annotations = annotations
	err := k8sClient.Create(testCtx, secret)
	Expect(err).NotTo(HaveOccurred())
}

func testGetSecret() *corev1.Secret {
	secret := &corev1.Secret{}
	err := k8sClient.Get(testCtx, testNSN, secret)
	Expect(err).NotTo(HaveOccurred())
	return secret
}
