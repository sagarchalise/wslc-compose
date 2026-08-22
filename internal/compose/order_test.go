package compose

import (
	"reflect"
	"testing"
)

func projectWithDeps(deps map[string][]string) *Project {
	p := &Project{Services: map[string]Service{}}
	for name, ds := range deps {
		dependsOn := map[string]DependsOn{}
		for _, d := range ds {
			dependsOn[d] = DependsOn{}
		}
		p.Services[name] = Service{Name: name, DependsOn: dependsOn}
	}
	return p
}

func indexOf(order []string, name string) int {
	for i, n := range order {
		if n == name {
			return i
		}
	}
	return -1
}

func TestStartOrderRespectsDependencies(t *testing.T) {
	p := projectWithDeps(map[string][]string{
		"db":  nil,
		"app": {"db"},
		"web": {"app"},
	})
	order, err := p.StartOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 3 {
		t.Fatalf("order = %v", order)
	}
	if indexOf(order, "db") > indexOf(order, "app") || indexOf(order, "app") > indexOf(order, "web") {
		t.Fatalf("dependency order violated: %v", order)
	}
}

func TestStopOrderIsReversed(t *testing.T) {
	p := projectWithDeps(map[string][]string{
		"db":  nil,
		"app": {"db"},
	})
	start, err := p.StartOrder()
	if err != nil {
		t.Fatal(err)
	}
	stop, err := p.StopOrder()
	if err != nil {
		t.Fatal(err)
	}
	reversedStart := append([]string{}, start...)
	for i, j := 0, len(reversedStart)-1; i < j; i, j = i+1, j-1 {
		reversedStart[i], reversedStart[j] = reversedStart[j], reversedStart[i]
	}
	if !reflect.DeepEqual(stop, reversedStart) {
		t.Fatalf("stop order %v is not the reverse of start order %v", stop, start)
	}
}

func TestStartOrderDetectsCycles(t *testing.T) {
	p := projectWithDeps(map[string][]string{
		"a": {"b"},
		"b": {"a"},
	})
	if _, err := p.StartOrder(); err == nil {
		t.Fatal("expected a circular dependency error")
	}
}
