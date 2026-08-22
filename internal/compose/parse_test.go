package compose

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeCompose(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadBasicProject(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
name: myproj
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
      - target: 9090
        published: 9091
        protocol: udp
    environment:
      FOO: bar
    labels:
      - "team=platform"
    networks:
      backend:
        aliases: [web-alias]
  db:
    image: postgres:16
    networks: [backend]

networks:
  backend:
    driver: bridge
`)

	proj, warnings, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if proj.Name != "myproj" {
		t.Fatalf("project name = %q, want myproj", proj.Name)
	}

	web := proj.Services["web"]
	if web.ContainerName != "myproj-web" {
		t.Errorf("container name = %q", web.ContainerName)
	}
	if len(web.Ports) != 2 || web.Ports[0].Raw != "8080:80" || web.Ports[1].Raw != "9091:9090/udp" {
		t.Errorf("ports = %+v", web.Ports)
	}
	if web.Environment["FOO"] != "bar" {
		t.Errorf("environment = %+v", web.Environment)
	}
	if web.Labels["team"] != "platform" {
		t.Errorf("labels = %+v", web.Labels)
	}
	attach, ok := web.Networks["backend"]
	if !ok || len(attach.Aliases) != 2 {
		t.Errorf("web networks = %+v", web.Networks)
	}

	net := proj.Networks["backend"]
	if net.Name != "myproj-backend" || net.Driver != "bridge" {
		t.Errorf("network = %+v", net)
	}

	if _, ok := proj.Networks["default"]; ok {
		t.Errorf("default network should not be created when all services declare networks")
	}
}

func TestLoadDefaultNetwork(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
name: myproj
services:
  web:
    image: nginx:latest
`)
	proj, _, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := proj.Networks["default"]; !ok {
		t.Fatalf("expected implicit default network")
	}
	if proj.Networks["default"].Name != "myproj-default" {
		t.Errorf("default network name = %q", proj.Networks["default"].Name)
	}
	attach := proj.Services["web"].Networks["default"]
	if len(attach.Aliases) != 1 || attach.Aliases[0] != "web" {
		t.Errorf("default network alias = %+v", attach)
	}
}

func TestLoadUnsupportedKeysWarn(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    image: nginx:latest
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "true"]
configs:
  foo:
    file: ./foo.txt
`)
	_, warnings, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) < 3 {
		t.Fatalf("expected warnings for restart, healthcheck, and configs, got %v", warnings)
	}
}

func TestLoadUndefinedNetworkErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    image: nginx:latest
    networks: [ghost]
`)
	if _, _, err := Load(path, ""); err == nil {
		t.Fatal("expected error for undefined network reference")
	}
}

func TestLoadUndefinedVolumeErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  web:
    image: nginx:latest
    volumes:
      - ghostvol:/data
`)
	if _, _, err := Load(path, ""); err == nil {
		t.Fatal("expected error for undefined volume reference")
	}
}

func TestLoadDependsOnConditionWarns(t *testing.T) {
	dir := t.TempDir()
	path := writeCompose(t, dir, `
services:
  db:
    image: postgres:16
  web:
    image: nginx:latest
    depends_on:
      db:
        condition: service_healthy
`)
	proj, warnings, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning about the unsupported health condition")
	}
	if _, ok := proj.Services["web"].DependsOn["db"]; !ok {
		t.Fatal("depends_on should still be recorded for ordering purposes")
	}
}

func TestLoadEnvFileMergedUnderExplicitEnvironment(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=from-file\nBAR=only-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeCompose(t, dir, `
services:
  web:
    image: nginx:latest
    env_file: .env
    environment:
      FOO: from-environment
`)
	proj, _, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	env := proj.Services["web"].Environment
	if env["FOO"] != "from-environment" {
		t.Errorf("environment: should override env_file, got FOO=%q", env["FOO"])
	}
	if env["BAR"] != "only-file" {
		t.Errorf("env_file-only var missing, got BAR=%q", env["BAR"])
	}
}

func TestLoadBuildDefaultsImageName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeCompose(t, dir, `
name: myproj
services:
  app:
    build: .
`)
	proj, _, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	app := proj.Services["app"]
	if app.Image != "myproj-app:latest" {
		t.Errorf("image = %q", app.Image)
	}
	if app.Build == nil || app.Build.Context != dir {
		t.Errorf("build context = %+v", app.Build)
	}
}

func TestResolveVolumeMountNamedVolume(t *testing.T) {
	volumes := map[string]Volume{"data": {Key: "data", Name: "proj-data"}}
	got, err := resolveVolumeMount("data:/var/lib/data", `C:\compose\dir`, volumes)
	if err != nil {
		t.Fatal(err)
	}
	if got != "proj-data:/var/lib/data" {
		t.Errorf("got %q", got)
	}
}

func TestResolveVolumeMountWindowsDriveLetter(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive-letter path semantics only apply on windows")
	}
	volumes := map[string]Volume{}
	got, err := resolveVolumeMount(`C:\host\path:/data:ro`, `C:\compose\dir`, volumes)
	if err != nil {
		t.Fatal(err)
	}
	if got != `C:\host\path:/data:ro` {
		t.Errorf("got %q", got)
	}
}

func TestResolveVolumeMountRelativeHostPath(t *testing.T) {
	volumes := map[string]Volume{}
	dir := t.TempDir()
	got, err := resolveVolumeMount("./data:/data", dir, volumes)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(filepath.Join(dir, "data")) + ":/data"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveVolumeMountUndefinedNamedVolume(t *testing.T) {
	if _, err := resolveVolumeMount("ghost:/data", t.TempDir(), map[string]Volume{}); err == nil {
		t.Fatal("expected an error for an undeclared volume reference")
	}
}
