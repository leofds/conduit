package config

// Local is the reserved host name that requests a localhost shell session.
const Local = "local"

type ResolverType string

const (
	ResolverFile ResolverType = "file"
	ResolverAPI  ResolverType = "api"
)

// ConduitConfigPaths is the ordered list of paths searched for conduit.yaml.
// Files are merged in order; later entries override earlier ones on duplicate keys.
var ConduitConfigPaths = []string{
	"/etc/conduit/conduit.yaml",
	"/etc/conduit/conduit.yml",
	"./conduit.yaml",
	"./conduit.yml",
}

// HostsConfigPaths is the ordered list of paths searched for hosts.yaml.
// Files are merged in order; later entries override earlier ones on duplicate keys.
var HostsConfigPaths = []string{
	"/etc/conduit/hosts.yaml",
	"/etc/conduit/hosts.yml",
	"./hosts.yaml",
	"./hosts.yml",
}
