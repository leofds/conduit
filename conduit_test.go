package conduit

import (
	"fmt"
	"testing"

	"github.com/leofds/conduit/knownhosts"
	"github.com/leofds/conduit/resolver"
)

type stubResolver struct{}

func (stubResolver) Resolve(_ resolver.Request) (resolver.SessionConfig, error) {
	return nil, fmt.Errorf("not found")
}

func TestNewRequiresResolverButDefaultsKnownHosts(t *testing.T) {
	s, err := New(Options{Resolver: stubResolver{}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s.Handler() == nil {
		t.Fatal("Handler() is nil")
	}
	if s.Sessions() == nil {
		t.Fatal("Sessions() is nil")
	}
}

func TestNormalizeKnownHostsNil(t *testing.T) {
	if _, ok := normalizeKnownHosts(nil).(*knownhosts.Memory); !ok {
		t.Fatalf("normalizeKnownHosts(nil) should return *Memory")
	}
}

func TestNormalizeKnownHostsTypedNil(t *testing.T) {
	var typedNil *knownhosts.Store
	if _, ok := normalizeKnownHosts(typedNil).(*knownhosts.Memory); !ok {
		t.Fatalf("normalizeKnownHosts(typed nil) should return *Memory")
	}
}

func TestNormalizeKnownHostsPreservesProvided(t *testing.T) {
	m := knownhosts.NewMemory()
	if got := normalizeKnownHosts(m); got != m {
		t.Fatalf("normalizeKnownHosts should return the provided provider")
	}
}
