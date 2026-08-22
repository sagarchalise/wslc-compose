// Package wslc is a thin exec wrapper around the wslc.exe CLI. It does not
// reimplement any container/network/volume logic itself -- every mutation
// goes through the real wslc binary. Runner is exported so callers (and
// tests) can substitute a fake instead of shelling out for real.
package wslc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const binary = "wslc"

const (
	LabelProject = "com.wslc-compose.project"
	LabelService = "com.wslc-compose.service"
	LabelNetwork = "com.wslc-compose.network"
	LabelVolume  = "com.wslc-compose.volume"
)

// Runner executes wslc subcommands. The real implementation is Client, which
// shells out to wslc.exe; tests substitute a fake that never touches it.
type Runner interface {
	Run(args ...string) error
	Capture(args ...string) (string, error)
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

// Run executes a wslc subcommand, streaming stdout/stderr/stdin directly.
func (c *Client) Run(args ...string) error {
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Capture executes a wslc subcommand and returns stdout without echoing it.
func (c *Client) Capture(args ...string) (string, error) {
	cmd := exec.Command(binary, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("%s: %w: %s", strings.Join(append([]string{binary}, args...), " "), err, strings.TrimSpace(errOut.String()))
	}
	return out.String(), nil
}

type inspectResult struct {
	Labels map[string]string `json:"Labels"`
}

func inspectLabels(r Runner, args ...string) (map[string]string, bool, error) {
	out, err := r.Capture(args...)
	if err != nil {
		// Any inspect failure is treated as "does not exist" for v1 -- a real
		// wslc problem (daemon down, etc.) will surface again on the next
		// mutating call anyway.
		return nil, false, nil
	}
	var results []inspectResult
	if err := json.Unmarshal([]byte(out), &results); err != nil || len(results) == 0 {
		return nil, false, nil
	}
	return results[0].Labels, true, nil
}

func NetworkLabels(r Runner, name string) (map[string]string, bool, error) {
	return inspectLabels(r, "network", "inspect", name)
}

func VolumeLabels(r Runner, name string) (map[string]string, bool, error) {
	return inspectLabels(r, "volume", "inspect", name)
}

func ContainerLabels(r Runner, name string) (map[string]string, bool, error) {
	return inspectLabels(r, "container", "inspect", name)
}

// Owns reports whether a resource's labels mark it as created by this project.
func Owns(labels map[string]string, project string) bool {
	return labels != nil && labels[LabelProject] == project
}
