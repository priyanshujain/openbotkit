package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/servertest"
	"github.com/73ai/openbotkit/remote"
	"github.com/73ai/openbotkit/service/memory"
	ansrc "github.com/73ai/openbotkit/source/applenotes"
	"github.com/73ai/openbotkit/store"
)

func newLocalBackend(t *testing.T) servertest.Backend {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("OBK_CONFIG_DIR", dir)

	cfg := config.Default()
	cfg.Auth = &config.AuthConfig{Username: "test", Password: "test"}

	for _, src := range []string{"gmail", "whatsapp", "history", "user_memory", "applenotes"} {
		if err := os.MkdirAll(filepath.Join(dir, src), 0700); err != nil {
			t.Fatalf("mkdir %s: %v", src, err)
		}
	}

	memDir := filepath.Join(dir, "user_memory")
	if err := memory.EnsureDir(memDir); err != nil {
		t.Fatalf("ensure memory dir: %v", err)
	}

	anDB, err := store.Open(store.Config{Driver: "sqlite", DSN: cfg.AppleNotesDataDSN()})
	if err != nil {
		t.Fatalf("open applenotes db: %v", err)
	}
	if err := ansrc.Migrate(anDB); err != nil {
		t.Fatalf("migrate applenotes: %v", err)
	}
	anDB.Close()

	s := &Server{cfg: cfg}
	mux := http.NewServeMux()
	s.routes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return servertest.Backend{
		Client:       remote.NewClient(ts.URL, "test", "test"),
		NoAuthClient: remote.NewClient(ts.URL, "", ""),
		SeedDB: func(t *testing.T, source, sql string) {
			t.Helper()
			dsn, err := cfg.SourceDataDSN(source)
			if err != nil {
				t.Fatalf("source DSN: %v", err)
			}
			if err := exec.Command("sqlite3", dsn, sql).Run(); err != nil {
				t.Fatalf("seed %s: %v", source, err)
			}
		},
	}
}

// TestServerAPI runs the server API contract tests (auth, CRUD, validation,
// DB proxy) against a local httptest server.
func TestServerAPI(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not in PATH")
	}
	b := newLocalBackend(t)
	servertest.Run(t, b)
}
