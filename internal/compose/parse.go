package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"wslc-compose/internal/naming"
)

type rawNetwork struct {
	Driver     string    `yaml:"driver"`
	DriverOpts stringMap `yaml:"driver_opts"`
	Labels     stringMap `yaml:"labels"`
	Name       string    `yaml:"name"`
	External   bool      `yaml:"external"`
}

type rawVolume struct {
	Driver     string    `yaml:"driver"`
	DriverOpts stringMap `yaml:"driver_opts"`
	Labels     stringMap `yaml:"labels"`
	Name       string    `yaml:"name"`
	External   bool      `yaml:"external"`
}

type rawService struct {
	Image         string           `yaml:"image"`
	Build         *buildSpec       `yaml:"build"`
	ContainerName string           `yaml:"container_name"`
	Command       stringList       `yaml:"command"`
	Entrypoint    stringList       `yaml:"entrypoint"`
	Environment   stringMap        `yaml:"environment"`
	EnvFile       stringList       `yaml:"env_file"`
	Ports         portList         `yaml:"ports"`
	Volumes       volumeEntryList  `yaml:"volumes"`
	Networks      networkAttachMap `yaml:"networks"`
	DependsOn     dependsOnMap     `yaml:"depends_on"`
	Labels        stringMap        `yaml:"labels"`
	Hostname      string           `yaml:"hostname"`
	Domainname    string           `yaml:"domainname"`
	User          string           `yaml:"user"`
	WorkingDir    string           `yaml:"working_dir"`
	DNS           stringList       `yaml:"dns"`
	DNSSearch     stringList       `yaml:"dns_search"`
	ShmSize       scalarString     `yaml:"shm_size"`
	MemLimit      scalarString     `yaml:"mem_limit"`
	CPUs          scalarString     `yaml:"cpus"`
	Tmpfs         stringList       `yaml:"tmpfs"`
	StopSignal    string           `yaml:"stop_signal"`
	Ulimits       ulimitMap        `yaml:"ulimits"`
}

type rawFile struct {
	Name     string                `yaml:"name"`
	Services map[string]rawService `yaml:"services"`
	Networks map[string]rawNetwork `yaml:"networks"`
	Volumes  map[string]rawVolume  `yaml:"volumes"`
}

var knownTopKeys = map[string]bool{"name": true, "services": true, "networks": true, "volumes": true}

var knownServiceKeys = map[string]bool{
	"image": true, "build": true, "container_name": true, "command": true, "entrypoint": true,
	"environment": true, "env_file": true, "ports": true, "volumes": true, "networks": true,
	"depends_on": true, "labels": true, "hostname": true, "domainname": true, "user": true,
	"working_dir": true, "dns": true, "dns_search": true, "shm_size": true, "mem_limit": true,
	"cpus": true, "tmpfs": true, "stop_signal": true, "ulimits": true,
}

var knownNetworkKeys = map[string]bool{
	"driver": true, "driver_opts": true, "labels": true, "name": true, "external": true,
}

var knownVolumeKeys = map[string]bool{
	"driver": true, "driver_opts": true, "labels": true, "name": true, "external": true,
}

func mappingKeys(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keys = append(keys, node.Content[i].Value)
	}
	return keys
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// Load parses a compose file at path and resolves it into a Project, using
// projectNameOverride (from -p) if non-empty. It returns human-readable
// warnings for compose-spec keys that wslc has no equivalent for.
func Load(path string, projectNameOverride string) (*Project, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(root.Content) == 0 {
		return nil, nil, fmt.Errorf("%s is empty", path)
	}
	doc := root.Content[0]

	var raw rawFile
	if err := doc.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var warnings []string
	warnUnknown := func(context string, node *yaml.Node, known map[string]bool) {
		for _, k := range mappingKeys(node) {
			if !known[k] {
				warnings = append(warnings, fmt.Sprintf("%s: unsupported key %q ignored", context, k))
			}
		}
	}

	warnUnknown(path, doc, knownTopKeys)
	if servicesNode := findKey(doc, "services"); servicesNode != nil {
		for i := 0; i < len(servicesNode.Content); i += 2 {
			name := servicesNode.Content[i].Value
			warnUnknown(fmt.Sprintf("service %q", name), servicesNode.Content[i+1], knownServiceKeys)
		}
	}
	if networksNode := findKey(doc, "networks"); networksNode != nil {
		for i := 0; i < len(networksNode.Content); i += 2 {
			name := networksNode.Content[i].Value
			warnUnknown(fmt.Sprintf("network %q", name), networksNode.Content[i+1], knownNetworkKeys)
		}
	}
	if volumesNode := findKey(doc, "volumes"); volumesNode != nil {
		for i := 0; i < len(volumesNode.Content); i += 2 {
			name := volumesNode.Content[i].Value
			warnUnknown(fmt.Sprintf("volume %q", name), volumesNode.Content[i+1], knownVolumeKeys)
		}
	}

	dir := filepath.Dir(path)
	project, err := buildProject(dir, raw, projectNameOverride, func(w string) {
		warnings = append(warnings, w)
	})
	if err != nil {
		return nil, warnings, err
	}
	return project, warnings, nil
}

func buildProject(dir string, raw rawFile, projectNameOverride string, warn func(string)) (*Project, error) {
	projectName := naming.ResolveProjectName(projectNameOverride, raw.Name, dir)

	project := &Project{
		Name:     projectName,
		Services: map[string]Service{},
		Networks: map[string]Network{},
		Volumes:  map[string]Volume{},
	}

	for key, rn := range raw.Networks {
		net := Network{
			Key:        key,
			Driver:     rn.Driver,
			DriverOpts: map[string]string(rn.DriverOpts),
			Labels:     map[string]string(rn.Labels),
			External:   rn.External,
		}
		switch {
		case rn.External:
			net.Name = rn.Name
			if net.Name == "" {
				net.Name = key
			}
		case rn.Name != "":
			net.Name = rn.Name
		default:
			net.Name = naming.Network(projectName, key)
		}
		project.Networks[key] = net
	}

	for key, rv := range raw.Volumes {
		vol := Volume{
			Key:        key,
			Driver:     rv.Driver,
			DriverOpts: map[string]string(rv.DriverOpts),
			Labels:     map[string]string(rv.Labels),
			External:   rv.External,
		}
		switch {
		case rv.External:
			vol.Name = rv.Name
			if vol.Name == "" {
				vol.Name = key
			}
		case rv.Name != "":
			vol.Name = rv.Name
		default:
			vol.Name = naming.Volume(projectName, key)
		}
		project.Volumes[key] = vol
	}

	serviceNames := make([]string, 0, len(raw.Services))
	for name := range raw.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	needsDefaultNetwork := false

	for _, name := range serviceNames {
		rs := raw.Services[name]

		if rs.Image == "" && rs.Build == nil {
			return nil, fmt.Errorf("service %q: must specify image or build", name)
		}

		svc := Service{
			Name:          name,
			Image:         rs.Image,
			ContainerName: rs.ContainerName,
			Command:       []string(rs.Command),
			Entrypoint:    []string(rs.Entrypoint),
			Labels:        map[string]string(rs.Labels),
			Hostname:      rs.Hostname,
			Domainname:    rs.Domainname,
			User:          rs.User,
			WorkingDir:    rs.WorkingDir,
			DNS:           []string(rs.DNS),
			DNSSearch:     []string(rs.DNSSearch),
			ShmSize:       string(rs.ShmSize),
			MemLimit:      string(rs.MemLimit),
			CPUs:          string(rs.CPUs),
			Tmpfs:         []string(rs.Tmpfs),
			StopSignal:    rs.StopSignal,
			Ulimits:       []string(rs.Ulimits),
		}

		if svc.ContainerName == "" {
			svc.ContainerName = naming.Service(projectName, name)
		}

		if rs.Build != nil {
			ctx := rs.Build.Context
			if !filepath.IsAbs(ctx) {
				ctx = filepath.Join(dir, ctx)
			}
			svc.Build = &BuildConfig{
				Context:    filepath.Clean(ctx),
				Dockerfile: rs.Build.Dockerfile,
				Args:       map[string]string(rs.Build.Args),
				Target:     rs.Build.Target,
			}
			if svc.Image == "" {
				svc.Image = naming.Service(projectName, name) + ":latest"
			}
		}

		env := map[string]string{}
		for _, ef := range rs.EnvFile {
			p := ef
			if !filepath.IsAbs(p) {
				p = filepath.Join(dir, p)
			}
			fileEnv, err := loadEnvFile(p)
			if err != nil {
				return nil, fmt.Errorf("service %q: env_file %q: %w", name, ef, err)
			}
			for k, v := range fileEnv {
				env[k] = v
			}
		}
		for k, v := range rs.Environment {
			env[k] = v
		}
		svc.Environment = env

		for _, p := range rs.Ports {
			svc.Ports = append(svc.Ports, PortMapping{Raw: p})
		}

		for _, rawMount := range rs.Volumes {
			resolved, err := resolveVolumeMount(rawMount, dir, project.Volumes)
			if err != nil {
				return nil, fmt.Errorf("service %q: %w", name, err)
			}
			svc.Volumes = append(svc.Volumes, VolumeMount{Raw: resolved})
		}

		if len(rs.Networks) == 0 {
			needsDefaultNetwork = true
			svc.Networks = map[string]NetworkAttachment{
				"default": {Aliases: []string{name}},
			}
		} else {
			svc.Networks = map[string]NetworkAttachment{}
			for netKey, attach := range rs.Networks {
				if netKey == "default" {
					needsDefaultNetwork = true
				} else if _, ok := project.Networks[netKey]; !ok {
					return nil, fmt.Errorf("service %q: refers to undefined network %q (declare it under top-level networks:)", name, netKey)
				}
				svc.Networks[netKey] = NetworkAttachment{Aliases: dedupe(append([]string{name}, attach.Aliases...))}
			}
		}

		svc.DependsOn = map[string]DependsOn{}
		for depName, dep := range rs.DependsOn {
			if _, ok := raw.Services[depName]; !ok {
				return nil, fmt.Errorf("service %q: depends_on refers to undefined service %q", name, depName)
			}
			if dep.Condition != "" && dep.Condition != "service_started" {
				warn(fmt.Sprintf("service %q: depends_on condition %q on %q is not supported by wslc; treating as plain start-order", name, dep.Condition, depName))
			}
			svc.DependsOn[depName] = dep
		}

		project.Services[name] = svc
	}

	if needsDefaultNetwork {
		if _, ok := project.Networks["default"]; !ok {
			project.Networks["default"] = Network{
				Key:    "default",
				Name:   naming.Network(projectName, "default"),
				Driver: "bridge",
			}
		}
	}

	return project, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func loadEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
		result[key] = val
	}
	return result, nil
}

func looksLikePath(s string) bool {
	if s == "." || s == ".." || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "\\") || strings.HasPrefix(s, "~") {
		return true
	}
	if len(s) >= 2 && s[1] == ':' {
		return true
	}
	return false
}

// splitVolumeSpec splits on ':' while treating a leading Windows drive letter
// (e.g. "C:\foo\bar:/data:ro") as part of the first segment, not a separator.
func splitVolumeSpec(raw string) []string {
	segments := strings.Split(raw, ":")
	if len(segments) > 1 && len(segments[0]) == 1 && isAlpha(segments[0][0]) {
		merged := segments[0] + ":" + segments[1]
		segments = append([]string{merged}, segments[2:]...)
	}
	return segments
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func resolveVolumeMount(raw string, composeDir string, volumes map[string]Volume) (string, error) {
	segments := splitVolumeSpec(raw)
	if len(segments) < 2 {
		return "", fmt.Errorf("volume entry %q: anonymous volumes (no host/volume source) are not supported", raw)
	}
	source := segments[0]
	rest := strings.Join(segments[1:], ":")

	if looksLikePath(source) {
		abs := source
		if !filepath.IsAbs(source) {
			abs = filepath.Join(composeDir, source)
		}
		return filepath.Clean(abs) + ":" + rest, nil
	}

	vol, ok := volumes[source]
	if !ok {
		return "", fmt.Errorf("volume entry %q: references undefined volume %q (declare it under top-level volumes:)", raw, source)
	}
	return vol.Name + ":" + rest, nil
}
