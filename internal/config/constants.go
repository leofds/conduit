package config

// Local is the reserved host name that requests a localhost shell session.
const Local = "local"

// SessionType identifies the kind of session in API request/response payloads.
type SessionType string

const (
	SessionTypeSSH   SessionType = "ssh"
	SessionTypeLocal SessionType = "local"
)
