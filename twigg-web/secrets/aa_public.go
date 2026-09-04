// Package secrets holds the entity of repo secrets.
package secrets

// A repo secret's id and name; never the secret value itself.
type SecretRef struct {
	Name string
	Id   uint64
}
