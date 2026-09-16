package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"

	cgoloader "github.com/compose-spec/compose-go/v2/loader"
	ctypes "github.com/compose-spec/compose-go/v2/types"

	"wslc-compose/internal/naming"
)

// Load parses a compose file at path and resolves it into a Project, using
// projectNameOverride (from -p) if non-empty. It returns human-readable
// warnings for compose-spec features that wslc has no equivalent for.
//
// Parsing itself is delegated to compose-go (the reference implementation
// used by docker compose v2), so the full compose-spec syntax is supported;
// this file only maps that result onto wslc's own domain model and flags
// features wslc's runtime doesn't implement.
func Load(path string, projectNameOverride string) (*Project, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	// compose-go fills in Name on every network/volume during normalization
	// (mirroring real docker compose's own project_key convention), so we
	// can't tell an explicit `name:` override apart from its auto-fill by
	// inspecting the loaded project alone. Peek the raw declarations here to
	// keep wslc's own dash-based naming convention for anything the user
	// didn't explicitly name.
	var head struct {
		Name     string                     `yaml:"name"`
		Networks map[string]rawResourceHead `yaml:"networks"`
		Volumes  map[string]rawResourceHead `yaml:"volumes"`
	}
	_ = yaml.Unmarshal(data, &head)

	dir := filepath.Dir(path)
	projectName := naming.ResolveProjectName(projectNameOverride, head.Name, dir)

	details := ctypes.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []ctypes.ConfigFile{{Filename: path, Content: data}},
		Environment: ctypes.NewMapping(os.Environ()),
	}

	cproject, err := cgoloader.LoadWithContext(context.Background(), details, func(o *cgoloader.Options) {
		o.SetProjectName(projectName, true)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var warnings []string
	warn := func(format string, args ...interface{}) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}

	project, err := buildProject(projectName, cproject, head.Networks, head.Volumes, warn)
	if err != nil {
		return nil, warnings, err
	}
	return project, warnings, nil
}

// rawResourceHead captures just enough of a top-level network/volume
// declaration to tell an explicit `name:` override apart from the name
// compose-go's normalization step auto-fills in for every resource.
type rawResourceHead struct {
	Name string `yaml:"name"`
}

func buildProject(projectName string, cproject *ctypes.Project, explicitNetworkNames, explicitVolumeNames map[string]rawResourceHead, warn func(string, ...interface{})) (*Project, error) {
	project := &Project{
		Name:     projectName,
		Services: map[string]Service{},
		Networks: map[string]Network{},
		Volumes:  map[string]Volume{},
	}

	for key, cn := range cproject.Networks {
		net := Network{
			Key:        key,
			Driver:     cn.Driver,
			DriverOpts: map[string]string(cn.DriverOpts),
			Labels:     map[string]string(cn.Labels),
			External:   bool(cn.External),
		}
		switch {
		case bool(cn.External):
			net.Name = cn.Name
			if net.Name == "" {
				net.Name = key
			}
		case explicitNetworkNames[key].Name != "":
			net.Name = explicitNetworkNames[key].Name
		default:
			net.Name = naming.Network(projectName, key)
		}
		project.Networks[key] = net
	}

	for key, cv := range cproject.Volumes {
		vol := Volume{
			Key:        key,
			Driver:     cv.Driver,
			DriverOpts: map[string]string(cv.DriverOpts),
			Labels:     map[string]string(cv.Labels),
			External:   bool(cv.External),
		}
		switch {
		case bool(cv.External):
			vol.Name = cv.Name
			if vol.Name == "" {
				vol.Name = key
			}
		case explicitVolumeNames[key].Name != "":
			vol.Name = explicitVolumeNames[key].Name
		default:
			vol.Name = naming.Volume(projectName, key)
		}
		project.Volumes[key] = vol
	}

	serviceNames := make([]string, 0, len(cproject.Services))
	for name := range cproject.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	for _, name := range serviceNames {
		rs := cproject.Services[name]

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
			Domainname:    rs.DomainName,
			User:          rs.User,
			WorkingDir:    rs.WorkingDir,
			DNS:           []string(rs.DNS),
			DNSSearch:     []string(rs.DNSSearch),
			Tmpfs:         []string(rs.Tmpfs),
			StopSignal:    rs.StopSignal,
		}

		if svc.ContainerName == "" {
			svc.ContainerName = naming.Service(projectName, name)
		}

		if rs.ShmSize > 0 {
			svc.ShmSize = strconv.FormatInt(int64(rs.ShmSize), 10)
		}
		if rs.MemLimit > 0 {
			svc.MemLimit = strconv.FormatInt(int64(rs.MemLimit), 10)
		}
		if rs.CPUS > 0 {
			svc.CPUs = strconv.FormatFloat(float64(rs.CPUS), 'f', -1, 32)
		}
		svc.Ulimits = formatUlimits(rs.Ulimits)

		if rs.Build != nil {
			ctx := rs.Build.Context
			svc.Build = &BuildConfig{
				Context:    filepath.Clean(ctx),
				Dockerfile: rs.Build.Dockerfile,
				Args:       stringPtrMapToMap(rs.Build.Args),
				Target:     rs.Build.Target,
			}
			if svc.Image == "" {
				svc.Image = naming.Service(projectName, name) + ":latest"
			}
		}

		svc.Environment = map[string]string{}
		for k, v := range rs.Environment {
			if v != nil {
				svc.Environment[k] = *v
			} else {
				svc.Environment[k] = ""
			}
		}

		for _, p := range rs.Ports {
			svc.Ports = append(svc.Ports, PortMapping{Raw: formatPort(p)})
		}

		for _, rawMount := range rs.Volumes {
			resolved, err := formatVolume(rawMount, project.Volumes)
			if err != nil {
				return nil, fmt.Errorf("service %q: %w", name, err)
			}
			svc.Volumes = append(svc.Volumes, VolumeMount{Raw: resolved})
		}

		svc.Networks = map[string]NetworkAttachment{}
		for netKey, attach := range rs.Networks {
			var aliases []string
			if attach != nil {
				aliases = attach.Aliases
			}
			svc.Networks[netKey] = NetworkAttachment{Aliases: dedupe(append([]string{name}, aliases...))}
		}

		svc.DependsOn = map[string]DependsOn{}
		for depName, dep := range rs.DependsOn {
			if dep.Condition != "" && dep.Condition != "service_started" {
				warn("service %q: depends_on condition %q on %q is not supported by wslc; treating as plain start-order", name, dep.Condition, depName)
			}
			svc.DependsOn[depName] = DependsOn{Condition: dep.Condition}
		}

		if rs.Restart != "" {
			warn("service %q: unsupported key \"restart\" ignored", name)
		}
		if rs.HealthCheck != nil {
			warn("service %q: unsupported key \"healthcheck\" ignored", name)
		}
		if rs.Deploy != nil {
			warn("service %q: unsupported key \"deploy\" ignored", name)
		}
		if len(rs.Secrets) > 0 {
			warn("service %q: unsupported key \"secrets\" ignored", name)
		}
		if len(rs.Configs) > 0 {
			warn("service %q: unsupported key \"configs\" ignored", name)
		}

		project.Services[name] = svc
	}

	if len(cproject.Configs) > 0 {
		warn("top-level: unsupported key \"configs\" ignored")
	}
	if len(cproject.Secrets) > 0 {
		warn("top-level: unsupported key \"secrets\" ignored")
	}

	return project, nil
}

func formatUlimits(limits map[string]*ctypes.UlimitsConfig) []string {
	if len(limits) == 0 {
		return nil
	}
	names := make([]string, 0, len(limits))
	for name := range limits {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]string, 0, len(names))
	for _, name := range names {
		lim := limits[name]
		if lim.Single != 0 {
			out = append(out, fmt.Sprintf("%s=%d", name, lim.Single))
		} else {
			out = append(out, fmt.Sprintf("%s=%d:%d", name, lim.Soft, lim.Hard))
		}
	}
	return out
}

func stringPtrMapToMap(m ctypes.MappingWithEquals) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if v != nil {
			out[k] = *v
		} else {
			out[k] = ""
		}
	}
	return out
}

// formatPort renders a compose-go port entry (already normalized from either
// short or long compose-spec syntax) as a string ready for `wslc run -p`.
func formatPort(p ctypes.ServicePortConfig) string {
	spec := strconv.FormatUint(uint64(p.Target), 10)
	if p.Published != "" {
		spec = p.Published + ":" + spec
	}
	if p.HostIP != "" {
		spec = p.HostIP + ":" + spec
	}
	if p.Protocol != "" && p.Protocol != "tcp" {
		spec += "/" + p.Protocol
	}
	return spec
}

// formatVolume renders a compose-go volume entry as a string ready for
// `wslc run -v`. Bind-mount host paths arrive already resolved to absolute
// paths by compose-go; named-volume sources are resolved to their generated
// container names here.
func formatVolume(v ctypes.ServiceVolumeConfig, volumes map[string]Volume) (string, error) {
	source := v.Source
	if v.Type == ctypes.VolumeTypeVolume {
		if source == "" {
			return "", fmt.Errorf("anonymous volumes (no host/volume source) are not supported")
		}
		vol, ok := volumes[source]
		if !ok {
			return "", fmt.Errorf("volume entry %q: references undefined volume %q (declare it under top-level volumes:)", v.String(), source)
		}
		source = vol.Name
	}

	spec := source + ":" + v.Target
	if v.ReadOnly {
		spec += ":ro"
	}
	return spec, nil
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
