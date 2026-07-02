package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// mockVault is a minimal in-memory fake of the Vault/OpenBao HTTP API covering
// the endpoints this provider uses.
type mockVault struct {
	server *httptest.Server

	mu      sync.Mutex
	mounts  map[string]map[string]interface{} // "litellm/" -> mount output
	configs map[string]map[string]interface{} // backend -> config data
	roles   map[string]map[string]interface{} // "backend/roles/name" -> role data
}

func newMockVault(t *testing.T) (*mockVault, *vaultClient) {
	t.Helper()
	m := &mockVault{
		mounts:  map[string]map[string]interface{}{},
		configs: map[string]map[string]interface{}{},
		roles:   map[string]map[string]interface{}{},
	}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	t.Cleanup(m.server.Close)

	client, err := newVaultClient(m.server.URL, "test-token")
	if err != nil {
		t.Fatalf("newVaultClient: %v", err)
	}
	return m, client
}

func (m *mockVault) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/v1/")

	switch {
	case path == "sys/mounts" && r.Method == http.MethodGet:
		writeSecret(w, map[string]interface{}{"data": toIface(m.mounts)})

	case strings.HasPrefix(path, "sys/mounts/"):
		mp := strings.TrimPrefix(path, "sys/mounts/")
		key := strings.TrimRight(mp, "/") + "/"
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			// Store the mount *output* shape (Vault returns int TTLs, not the
			// string TTLs it accepts on input).
			m.mounts[key] = map[string]interface{}{
				"type":        body["type"],
				"description": body["description"],
				"config":      map[string]interface{}{},
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			delete(m.mounts, key)
			w.WriteHeader(http.StatusNoContent)
		}

	case strings.HasSuffix(path, "/config"):
		backend := strings.TrimSuffix(path, "/config")
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			m.configs[backend] = body
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			cfg, ok := m.configs[backend]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			// The real backend never returns master_key.
			out := map[string]interface{}{}
			for k, v := range cfg {
				if k == "master_key" {
					continue
				}
				out[k] = v
			}
			writeSecret(w, map[string]interface{}{"data": out})
		}

	case strings.Contains(path, "/roles/"):
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			m.roles[path] = body
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			role, ok := m.roles[path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeSecret(w, map[string]interface{}{"data": role})
		case http.MethodDelete:
			delete(m.roles, path)
			w.WriteHeader(http.StatusNoContent)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func toIface(m map[string]map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func writeSecret(w http.ResponseWriter, body map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(body)
}

func TestClient_MountLifecycle(t *testing.T) {
	_, c := newMockVault(t)
	ctx := context.Background()

	if err := c.enableMount(ctx, "litellm", "vault-plugin-secrets-litellm", "desc", mountConfig{}); err != nil {
		t.Fatalf("enableMount: %v", err)
	}
	info, err := c.mountInfo(ctx, "litellm")
	if err != nil {
		t.Fatalf("mountInfo: %v", err)
	}
	if info == nil {
		t.Fatal("expected mount to exist")
	}
	if info.Type != "vault-plugin-secrets-litellm" {
		t.Fatalf("unexpected mount type: %q", info.Type)
	}

	if err := c.disableMount(ctx, "litellm"); err != nil {
		t.Fatalf("disableMount: %v", err)
	}
	info, err = c.mountInfo(ctx, "litellm")
	if err != nil {
		t.Fatalf("mountInfo after delete: %v", err)
	}
	if info != nil {
		t.Fatal("expected mount to be gone")
	}
}

func TestClient_ConfigRoundTrip(t *testing.T) {
	m, c := newMockVault(t)
	ctx := context.Background()

	err := c.writeConfig(ctx, "litellm", map[string]interface{}{
		"base_url":                "http://litellm:4000",
		"master_key":              "sk-secret",
		"request_timeout_seconds": 30,
	})
	if err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	cfg, err := c.readConfig(ctx, "litellm")
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if cfg["base_url"] != "http://litellm:4000" {
		t.Fatalf("unexpected base_url: %v", cfg["base_url"])
	}
	if _, leaked := cfg["master_key"]; leaked {
		t.Fatal("master_key must not be returned by read")
	}
	_ = m
}

func TestClient_ReadConfigMissing(t *testing.T) {
	_, c := newMockVault(t)
	cfg, err := c.readConfig(context.Background(), "litellm")
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got %v", cfg)
	}
}

func TestClient_RoleLifecycle(t *testing.T) {
	_, c := newMockVault(t)
	ctx := context.Background()

	err := c.writeRole(ctx, "litellm", "team-a", map[string]interface{}{
		"models":     []string{"gpt-4"},
		"max_budget": 25.0,
		"ttl":        3600,
	})
	if err != nil {
		t.Fatalf("writeRole: %v", err)
	}

	role, err := c.readRole(ctx, "litellm", "team-a")
	if err != nil {
		t.Fatalf("readRole: %v", err)
	}
	if role == nil {
		t.Fatal("expected role to exist")
	}

	if err := c.deleteRole(ctx, "litellm", "team-a"); err != nil {
		t.Fatalf("deleteRole: %v", err)
	}
	role, err = c.readRole(ctx, "litellm", "team-a")
	if err != nil {
		t.Fatalf("readRole after delete: %v", err)
	}
	if role != nil {
		t.Fatal("expected role to be gone")
	}
}

func TestRolePath(t *testing.T) {
	if got := rolePath("litellm/", "team-a"); got != "litellm/roles/team-a" {
		t.Fatalf("unexpected role path: %q", got)
	}
}
