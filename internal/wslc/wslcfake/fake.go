// Package wslcfake is a test double for wslc.Runner. It never shells out to
// the real wslc.exe, so tests using it run identically on Linux (e.g. inside
// the devcontainer) and on Windows.
package wslcfake

import (
	"encoding/json"
	"errors"
	"strings"
)

type Call struct {
	Args []string
}

func (c Call) String() string {
	return strings.Join(c.Args, " ")
}

type Runner struct {
	Calls []Call

	captureResponses map[string]string
	captureErrors    map[string]error
	runErrors        map[string]error
}

func New() *Runner {
	return &Runner{
		captureResponses: map[string]string{},
		captureErrors:    map[string]error{},
		runErrors:        map[string]error{},
	}
}

func key(args []string) string {
	return strings.Join(args, " ")
}

func (f *Runner) Run(args ...string) error {
	f.Calls = append(f.Calls, Call{Args: append([]string{}, args...)})
	return f.runErrors[key(args)]
}

func (f *Runner) Capture(args ...string) (string, error) {
	f.Calls = append(f.Calls, Call{Args: append([]string{}, args...)})
	k := key(args)
	if err, ok := f.captureErrors[k]; ok {
		return "", err
	}
	return f.captureResponses[k], nil
}

// SetLabels makes `wslc <kind> inspect <name>` behave as if an object with
// the given labels already exists.
func (f *Runner) SetLabels(kind, name string, labels map[string]string) {
	data, _ := json.Marshal([]map[string]interface{}{{"Labels": labels}})
	f.captureResponses[key([]string{kind, "inspect", name})] = string(data)
}

// SetNotFound makes `wslc <kind> inspect <name>` behave as if no such object exists.
func (f *Runner) SetNotFound(kind, name string) {
	f.captureErrors[key([]string{kind, "inspect", name})] = errors.New("no such object")
}

// SetRunError makes the given Run invocation fail.
func (f *Runner) SetRunError(err error, args ...string) {
	f.runErrors[key(args)] = err
}

// CallsMatching returns every recorded call whose args start with prefix.
func (f *Runner) CallsMatching(prefix ...string) []Call {
	var out []Call
	for _, c := range f.Calls {
		if len(c.Args) < len(prefix) {
			continue
		}
		match := true
		for i, p := range prefix {
			if c.Args[i] != p {
				match = false
				break
			}
		}
		if match {
			out = append(out, c)
		}
	}
	return out
}
