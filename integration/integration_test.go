//go:build windows

// Package integration exercises wslc-compose against the real wslc.exe
// binary. It only compiles on windows (wslc.exe doesn't exist elsewhere) and
// skips at runtime if wslc isn't on PATH. Everything else in this module is
// tested with wslcfake, which runs the same on Linux (e.g. the devcontainer)
// and on Windows.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"wslc-compose/internal/cli"
	"wslc-compose/internal/compose"
	"wslc-compose/internal/wslc"
)

func requireWslc(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("wslc"); err != nil {
		t.Skip("wslc.exe not found on PATH; skipping integration test")
	}
}

func TestUpDownLifecycleAgainstRealWslc(t *testing.T) {
	requireWslc(t)

	dir := t.TempDir()
	composeContent := `
name: wslccomposeit
services:
  probe:
    image: alpine
    command: ["sleep", "3600"]
`
	composePath := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	proj, _, err := compose.Load(composePath, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	client := wslc.NewClient()
	t.Cleanup(func() { _ = cli.Down(client, proj) })

	if err := cli.Up(client, proj); err != nil {
		t.Fatalf("Up: %v", err)
	}

	containerName := proj.Services["probe"].ContainerName
	labels, exists, err := wslc.ContainerLabels(client, containerName)
	if err != nil {
		t.Fatalf("ContainerLabels: %v", err)
	}
	if !exists {
		t.Fatal("expected container to exist after Up")
	}
	if !wslc.Owns(labels, proj.Name) {
		t.Fatalf("container missing ownership label: %v", labels)
	}

	netLabels, netExists, err := wslc.NetworkLabels(client, proj.Networks["default"].Name)
	if err != nil {
		t.Fatalf("NetworkLabels: %v", err)
	}
	if !netExists || !wslc.Owns(netLabels, proj.Name) {
		t.Fatalf("expected default network to exist and be owned by the project")
	}

	if err := cli.Down(client, proj); err != nil {
		t.Fatalf("Down: %v", err)
	}

	if _, exists, _ := wslc.ContainerLabels(client, containerName); exists {
		t.Fatal("expected container to be removed after Down")
	}
	if _, exists, _ := wslc.NetworkLabels(client, proj.Networks["default"].Name); exists {
		t.Fatal("expected default network to be removed after Down")
	}
}
