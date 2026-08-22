package cli

import (
	"testing"

	"wslc-compose/internal/wslc"
	"wslc-compose/internal/wslc/wslcfake"
)

func TestStartStartsExistingOwnedContainersInDependencyOrder(t *testing.T) {
	fake := wslcfake.New()
	fake.SetLabels("container", "proj-db", map[string]string{wslc.LabelProject: "proj"})
	fake.SetLabels("container", "proj-app", map[string]string{wslc.LabelProject: "proj"})

	if err := Start(fake, sampleProject()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	calls := fake.CallsMatching("start")
	if len(calls) != 2 || calls[0].Args[1] != "proj-db" || calls[1].Args[1] != "proj-app" {
		t.Fatalf("start order = %v, want db before app", calls)
	}
}

func TestStartSkipsMissingContainer(t *testing.T) {
	fake := wslcfake.New()
	fake.SetNotFound("container", "proj-db")
	fake.SetNotFound("container", "proj-app")

	if err := Start(fake, sampleProject()); err != nil {
		t.Fatalf("Start should skip missing containers rather than erroring: %v", err)
	}
	if calls := fake.CallsMatching("start"); len(calls) != 0 {
		t.Fatalf("expected no start calls, got %v", calls)
	}
}

func TestStopStopsExistingOwnedContainersInReverseOrder(t *testing.T) {
	fake := wslcfake.New()
	fake.SetLabels("container", "proj-db", map[string]string{wslc.LabelProject: "proj"})
	fake.SetLabels("container", "proj-app", map[string]string{wslc.LabelProject: "proj"})

	if err := Stop(fake, sampleProject()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	calls := fake.CallsMatching("stop")
	if len(calls) != 2 || calls[0].Args[1] != "proj-app" || calls[1].Args[1] != "proj-db" {
		t.Fatalf("stop order = %v, want app before db", calls)
	}
}
