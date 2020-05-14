module github.com/mmlt/vault-secret

go 1.13

require (
	github.com/hashicorp/vault v1.4.1
	github.com/hashicorp/vault-plugin-auth-kubernetes v0.6.1
	github.com/hashicorp/vault/api v1.0.5-0.20200317185738-82f498082f02
	github.com/hashicorp/vault/sdk v0.1.14-0.20200429182704-29fce8f27ce4
	github.com/mmlt/testr v0.0.0-20200331071714-d38912dd7e5a
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/stretchr/testify v1.4.0
	k8s.io/api v0.17.5
	k8s.io/apimachinery v0.17.5
	k8s.io/client-go v0.17.5
	sigs.k8s.io/controller-runtime v0.5.2
)
