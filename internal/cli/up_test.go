package cli

import (
	"strings"
	"testing"

	"wslc-compose/internal/compose"
	"wslc-compose/internal/wslc"
	"wslc-compose/internal/wslc/wslcfake"
)

func sampleProject() *compose.Project {
	return &compose.Project{
		Name: "proj",
		Networks: map[string]compose.Network{
			"backend": {Key: "backend", Name: "proj-backend", Driver: "bridge"},
		},
		Volumes: map[string]compose.Volume{
			"data": {Key: "data", Name: "proj-data"},
		},
		Services: map[string]compose.Service{
			"db": {
				Name:          "db",
				Image:         "postgres:16",
				ContainerName: "proj-db",
				Environment:   map[string]string{"POSTGRES_PASSWORD": "test"},
				Volumes:       []compose.VolumeMount{{Raw: "proj-data:/var/lib/postgresql/data"}},
				Networks:      map[string]compose.NetworkAttachment{"backend": {Aliases: []string{"db"}}},
				DependsOn:     map[string]compose.DependsOn{},
			},
			"app": {
				Name:          "app",
				Image:         "alpine",
				ContainerName: "proj-app",
				Networks:      map[string]compose.NetworkAttachment{"backend": {Aliases: []string{"app"}}},
				DependsOn:     map[string]compose.DependsOn{"db": {}},
			},
		},
	}
}

func TestUpCreatesNetworkVolumeAndContainersInOrder(t *testing.T) {
	fake := wslcfake.New()
	fake.SetNotFound("network", "proj-backend")
	fake.SetNotFound("volume", "proj-data")
	fake.SetNotFound("container", "proj-db")
	fake.SetNotFound("container", "proj-app")

	if err := Up(fake, sampleProject()); err != nil {
		t.Fatalf("Up: %v", err)
	}

	netCalls := fake.CallsMatching("network", "create")
	if len(netCalls) != 1 || netCalls[0].Args[len(netCalls[0].Args)-1] != "proj-backend" {
		t.Fatalf("network create calls = %v", netCalls)
	}

	volCalls := fake.CallsMatching("volume", "create")
	if len(volCalls) != 1 || volCalls[0].Args[len(volCalls[0].Args)-1] != "proj-data" {
		t.Fatalf("volume create calls = %v", volCalls)
	}

	runCalls := fake.CallsMatching("run")
	if len(runCalls) != 2 {
		t.Fatalf("expected 2 run calls, got %v", runCalls)
	}
	if !containsArg(runCalls[0].Args, "proj-db") {
		t.Errorf("db should be started before app: %v", runCalls)
	}
	if !containsArg(runCalls[1].Args, "proj-app") {
		t.Errorf("app run call = %v", runCalls[1].Args)
	}
	appArgs := strings.Join(runCalls[1].Args, " ")
	if !strings.Contains(appArgs, "--network proj-backend") || !strings.Contains(appArgs, "--network-alias app") {
		t.Errorf("app run args missing network/alias: %q", appArgs)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestUpReusesOwnedExistingContainer(t *testing.T) {
	fake := wslcfake.New()
	fake.SetNotFound("network", "proj-backend")
	fake.SetNotFound("volume", "proj-data")
	fake.SetLabels("container", "proj-db", map[string]string{wslc.LabelProject: "proj"})
	fake.SetLabels("container", "proj-app", map[string]string{wslc.LabelProject: "proj"})

	if err := Up(fake, sampleProject()); err != nil {
		t.Fatalf("Up: %v", err)
	}

	if calls := fake.CallsMatching("run"); len(calls) != 0 {
		t.Fatalf("expected no run calls for existing owned containers, got %v", calls)
	}
	startCalls := fake.CallsMatching("start")
	if len(startCalls) != 2 {
		t.Fatalf("expected 2 start calls, got %v", startCalls)
	}
}

func TestUpRefusesUnownedExistingContainer(t *testing.T) {
	fake := wslcfake.New()
	fake.SetNotFound("network", "proj-backend")
	fake.SetNotFound("volume", "proj-data")
	fake.SetLabels("container", "proj-db", map[string]string{wslc.LabelProject: "someone-else"})

	err := Up(fake, sampleProject())
	if err == nil {
		t.Fatal("expected an error for a name collision with a container this project doesn't own")
	}
}
