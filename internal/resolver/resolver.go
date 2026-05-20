package resolver

// SessionConfig is the sealed parent interface for all session configurations.
// Use SSHConfig or LocalConfig as concrete implementations.
// The unexported marker method prevents implementations outside this package.
type SessionConfig interface {
	isSessionConfig()
}

// SSHConfig holds the parameters for an SSH session.
type SSHConfig struct {
	Address  string
	Port     string
	Username string
	Password string
	Term     string
}

func (SSHConfig) isSessionConfig() {}

// LocalConfig holds the parameters for a local shell session.
type LocalConfig struct {
	Command string
	Term    string
}

func (LocalConfig) isSessionConfig() {}

// Request carries the identifying information sent by the client.
type Request struct {
	Host  string
	Token string
}

// Resolver looks up the session configuration for a given request.
// Implement this interface to plug in your own validation logic — database
// lookups, external API calls, LDAP queries, etc.
type Resolver interface {
	Resolve(req Request) (SessionConfig, error)
}
