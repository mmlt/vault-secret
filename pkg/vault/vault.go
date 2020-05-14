package vault

type Loginer interface {
	// Login vault
	Login(authPath, role string) (Getter, error)
}

type Getter interface {
	// Get values from vault.
	Get(path string) (map[string]string, error)
}
