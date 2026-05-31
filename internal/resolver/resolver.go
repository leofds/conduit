package resolver

import "time"

// SessionConfig is the sealed parent interface for all session configurations.
// Use SSHConfig or LocalConfig as concrete implementations.
// The unexported marker method prevents implementations outside this package.
type SessionConfig interface {
	isSessionConfig()
}

// SSHConfig holds the parameters for an SSH session.
type SSHConfig struct {
	Address           string
	Port              string
	Username          string
	Password          string
	PrivateKeyFile    string
	Term              string
	Env               map[string]string
	TOFUAutoAccept    *bool          // per-host override; nil = use global default
	VerifyHostKey     *bool          // per-host override; nil = use global default
	IdleTimeout       *time.Duration // per-host override; nil = use global default
	KeepaliveInterval *time.Duration // per-host override; nil = use global default
}

func (SSHConfig) isSessionConfig() {}

// LocalConfig holds the parameters for a local shell session.
type LocalConfig struct {
	Command     string
	Term        string
	WorkingDir  *string           // per-session override; nil = use global default
	IdleTimeout *time.Duration    // per-session override; nil = use global default
	Env         map[string]string // additional environment variables for the local session
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
