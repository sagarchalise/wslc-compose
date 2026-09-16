package naming

import "testing"

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"MyProject":     "myproject",
		"my_project":    "my_project",
		"My Project!!":  "my-project",
		"---leading":    "leading",
		"trailing---":   "trailing",
		"":              "wslc-compose",
		"Ünïcode Stuff": "n-code-stuff",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveProjectName(t *testing.T) {
	if got := ResolveProjectName("Flag", "FileName", "/some/dir"); got != "flag" {
		t.Errorf("flag should take precedence, got %q", got)
	}
	if got := ResolveProjectName("", "FileName", "/some/dir"); got != "filename" {
		t.Errorf("name: key should take precedence over directory, got %q", got)
	}
	if got := ResolveProjectName("", "", "/some/My Dir"); got != "my-dir" {
		t.Errorf("should fall back to sanitized directory basename, got %q", got)
	}
}

func TestResourceNames(t *testing.T) {
	if got := Service("proj", "web"); got != "proj-web" {
		t.Errorf("Service() = %q", got)
	}
	if got := Network("proj", "backend"); got != "proj-backend" {
		t.Errorf("Network() = %q", got)
	}
	if got := Volume("proj", "data"); got != "proj-data" {
		t.Errorf("Volume() = %q", got)
	}
}
