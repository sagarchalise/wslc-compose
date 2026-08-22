package cli

import (
	"testing"

	"wslc-compose/internal/wslc"
	"wslc-compose/internal/wslc/wslcfake"
)

func TestDownStopsRemovesInReverseOrderAndKeepsVolumes(t *testing.T) {
	fake := wslcfake.New()
	fake.SetLabels("container", "proj-db", map[string]string{wslc.LabelProject: "proj"})
	fake.SetLabels("container", "proj-app", map[string]string{wslc.LabelProject: "proj"})
	fake.SetLabels("network", "proj-backend", map[string]string{wslc.LabelProject: "proj"})

	if err := Down(fake, sampleProject()); err != nil {
		t.Fatalf("Down: %v", err)
	}

	stopCalls := fake.CallsMatching("stop")
	if len(stopCalls) != 2 || stopCalls[0].Args[1] != "proj-app" || stopCalls[1].Args[1] != "proj-db" {
		t.Fatalf("stop order = %v, want app before db (reverse of start order)", stopCalls)
	}

	removeCalls := fake.CallsMatching("remove")
	if len(removeCalls) != 2 {
		t.Fatalf("expected 2 remove calls, got %v", removeCalls)
	}

	netRemoveCalls := fake.CallsMatching("network", "remove")
	if len(netRemoveCalls) != 1 || netRemoveCalls[0].Args[2] != "proj-backend" {
		t.Fatalf("network remove calls = %v", netRemoveCalls)
	}

	if calls := fake.CallsMatching("volume", "remove"); len(calls) != 0 {
		t.Fatalf("down should never remove volumes, got %v", calls)
	}
}

func TestDownSkipsContainersNotOwnedByProject(t *testing.T) {
	fake := wslcfake.New()
	fake.SetLabels("container", "proj-db", map[string]string{wslc.LabelProject: "someone-else"})
	fake.SetNotFound("container", "proj-app")
	fake.SetNotFound("network", "proj-backend")

	if err := Down(fake, sampleProject()); err != nil {
		t.Fatalf("Down should skip, not error, on unowned/missing resources: %v", err)
	}
	if calls := fake.CallsMatching("stop"); len(calls) != 0 {
		t.Fatalf("expected no stop calls, got %v", calls)
	}
	if calls := fake.CallsMatching("remove"); len(calls) != 0 {
		t.Fatalf("expected no remove calls, got %v", calls)
	}
}
