package controllers

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func testCreateSecret(t *testing.T, annotations map[string]string, data map[string][]byte) {
	t.Helper()

	testDeleteSecret(t)
	secret := &corev1.Secret{}
	secret.Namespace = testNSN.Namespace
	secret.Name = testNSN.Name
	secret.Data = data
	secret.Annotations = annotations
	err := k8sClient.Create(testCtx, secret)
	assert.NoError(t, err)
}

func testDeleteSecret(t *testing.T) {
	t.Helper()

	obj := &corev1.Secret{}
	err := k8sClient.Get(testCtx, testNSN, obj)
	if apierrors.IsNotFound(err) {
		return
	}
	assert.NoError(t, err)
	err = k8sClient.Delete(testCtx, obj)
	assert.NoError(t, err)
}

func testGetSecret(t *testing.T) *corev1.Secret {
	t.Helper()

	secret := &corev1.Secret{}
	err := k8sClient.Get(testCtx, testNSN, secret)
	assert.NoError(t, err)
	return secret
}

func testCreateServiceAccount(t *testing.T, namespace, name string) {
	t.Helper()

	nsn := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	obj := &corev1.ServiceAccount{}
	err := k8sClient.Get(testCtx, nsn, obj)
	if apierrors.IsNotFound(err) {
		obj.Namespace = namespace
		obj.Name = name
		err = k8sClient.Create(testCtx, obj)
		assert.NoError(t, err)
	}
	assert.NoError(t, err)
}

func testGetServiceAccountToken(t *testing.T, namespace, name string) string {
	t.Helper()

	nsn := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	sa := &corev1.ServiceAccount{}
	err := k8sClient.Get(testCtx, nsn, sa)
	if !assert.NoError(t, err) {
		return ""
	}

	if !assert.NotEmpty(t, sa.Secrets, "expected at least 1 secret") {
		return ""
	}
	nsn.Name = sa.Secrets[0].Name // SA and Secret are in the same namespace

	sc := &corev1.Secret{}
	err = k8sClient.Get(testCtx, nsn, sc)
	if !assert.NoError(t, err) {
		return ""
	}

	token, ok := sc.Data["token"]
	assert.True(t, ok, "expected 'token' field")

	return string(token)
}

func msb2mss(in map[string][]byte) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = string(v)
	}
	return out
}
