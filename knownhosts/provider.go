package knownhosts

// Provider is the storage backend for TOFU host key fingerprints.
type Provider interface {
	// Get returns the stored SHA256 fingerprint for host, or an empty string
	// when the host is not yet known.
	Get(host string) string
	// Set persists (or overwrites) the fingerprint for host.
	Set(host, fingerprint string) error
	// Remove deletes the stored fingerprint for host. It is not an error if the
	// host is unknown. Used by the CLI's -R / --reset-known-host flag.
	Remove(host string) error
}

// Compile-time guarantees that the built-in stores implement Provider, so they
// can be used wherever a Provider is expected (embedding and CLI).
var (
	_ Provider = (*Store)(nil)
	_ Provider = (*Memory)(nil)
)
