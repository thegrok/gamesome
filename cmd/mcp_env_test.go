package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRenderLaunchEnv(t *testing.T) {
	t.Setenv("GAMESOM_TEST_MARKER", "marker-value")

	var buf bytes.Buffer
	if err := renderLaunchEnv(&buf); err != nil {
		t.Fatalf("renderLaunchEnv: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "WARNING: may contain secrets") {
		t.Errorf("dump missing secrets warning header:\n%s", out)
	}
	if !strings.Contains(out, "GAMESOM_TEST_MARKER=marker-value") {
		t.Errorf("dump missing environment variable line:\n%s", out)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if !strings.Contains(out, "cwd: "+cwd) {
		t.Errorf("dump missing cwd line (want %q):\n%s", cwd, out)
	}

	// The env block (everything after the separator) must be sorted so two
	// dumps diff cleanly.
	_, envBlock, found := strings.Cut(out, "--- environment (sorted) ---\n")
	if !found {
		t.Fatalf("dump missing environment separator:\n%s", out)
	}
	envLines := strings.Split(strings.TrimSpace(envBlock), "\n")
	if !sort.StringsAreSorted(envLines) {
		t.Errorf("environment block is not sorted")
	}
}

func TestWriteLaunchEnvDump(t *testing.T) {
	// XDG_DATA_HOME overrides dataDir on every OS — the documented
	// hermetic-test hook, so the dump never touches the real data dir.
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	path, err := writeLaunchEnvDump()
	if err != nil {
		t.Fatalf("writeLaunchEnvDump: %v", err)
	}
	if filepath.Dir(path) != filepath.Join(os.Getenv("XDG_DATA_HOME"), "gamesom") {
		t.Errorf("dump written to %q, want under XDG_DATA_HOME/gamesom", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	if !strings.Contains(string(content), "gamesom mcp launch-environment dump") {
		t.Errorf("dump file missing header:\n%s", content)
	}
}
