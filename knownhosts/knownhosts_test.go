package knownhosts

import "testing"

func TestMemoryProvider(t *testing.T) {
	m := NewMemory()

	if got := m.Get("host1"); got != "" {
		t.Fatalf("Get(host1) = %q, want empty", got)
	}

	if err := m.Set("host1", "SHA256:abc"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if got := m.Get("host1"); got != "SHA256:abc" {
		t.Fatalf("Get(host1) = %q, want SHA256:abc", got)
	}
}

func TestStoreImplementsProvider(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir + "/known_hosts.yaml")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var p Provider = s

	if err := p.Set("h", "fp"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if got := p.Get("h"); got != "fp" {
		t.Fatalf("Get() = %q, want fp", got)
	}
	if err := p.Remove("h"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if got := p.Get("h"); got != "" {
		t.Fatalf("Get() after Remove = %q, want empty", got)
	}
}

func TestMemoryRemove(t *testing.T) {
	m := NewMemory()
	if err := m.Set("h", "fp"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := m.Remove("h"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if got := m.Get("h"); got != "" {
		t.Fatalf("Get() after Remove = %q, want empty", got)
	}
}
