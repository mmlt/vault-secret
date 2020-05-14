package controllers

import (
	"github.com/mmlt/testr"
	"github.com/mmlt/vault-secret/pkg/vault"
	"github.com/stretchr/testify/assert"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
	"time"
)

func TestFakevault(t *testing.T) {
	stop := make(chan struct{})

	logf.SetLogger(testr.New(t))

	testManager(t, fakeVault(map[string]string{
		"one": "first-value",
		"two": "second-value",
	}), stop)

	t.Run("should_not_change_Secret_that_is_not_annotated", func(t *testing.T) {
		testCreateSecret(t, nil)
		got := testGetSecret(t)
		assert.Equal(t, 1, len(got.Data))
	})

	t.Run("should_set_data_fields_when_Secret_is_annotated", func(t *testing.T) {
		testCreateSecret(t, map[string]string{
			"vault.mmlt.nl/inject":        "true",
			"vault.mmlt.nl/inject-path":   "path/to/secret",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		})
		got := testGetSecret(t)
		assert.Equal(t, 3, len(got.Data))
		assert.Equal(t, map[string]string{
			"een":             "first-value",
			"twee":            "second-value",
			"shouldNotChange": "value",
		}, msb2mss(got.Data))
	})

	t.Run("should_not_change_Secret_when_inject=false", func(t *testing.T) {
		testCreateSecret(t, map[string]string{
			"vault.mmlt.nl/inject":        "false",
			"vault.mmlt.nl/inject-path":   "path/to/secret",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		})
		got := testGetSecret(t)
		assert.Equal(t, 1, len(got.Data))
	})

	t.Run("should_not_change_Secret_when_inject_is_missing", func(t *testing.T) {
		testCreateSecret(t, map[string]string{
			"vault.mmlt.nl/inject-path":   "path/to/secret",
			"vault.mmlt.nl/inject-fields": "een=one,twee=two",
		})
		got := testGetSecret(t)
		assert.Equal(t, 1, len(got.Data))
	})

	// teardown manager
	close(stop)
	time.Sleep(time.Second) //TODO how to wait for manager shutdown?
}

type fakeVault map[string]string

func (v fakeVault) Login(_, _ string) (vault.Getter, error) {
	return v, nil
}

func (v fakeVault) Get(_ string) (map[string]string, error) {
	return v, nil
}
