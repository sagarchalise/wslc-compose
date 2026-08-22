package compose

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func isNull(node *yaml.Node) bool {
	return node.Tag == "!!null"
}

// stringList accepts either a scalar string or a sequence of strings
// (used for command, entrypoint, dns, dns_search, env_file, tmpfs).
type stringList []string

func (s *stringList) UnmarshalYAML(node *yaml.Node) error {
	if isNull(node) {
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		var str string
		if err := node.Decode(&str); err != nil {
			return err
		}
		*s = []string{str}
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		*s = list
	default:
		return fmt.Errorf("expected a string or list of strings, got %v", node.Kind)
	}
	return nil
}

// scalarString accepts any scalar (string, int, float, bool) and keeps its
// literal text -- compose files often leave things like `cpus: 0.5` unquoted.
type scalarString string

func (s *scalarString) UnmarshalYAML(node *yaml.Node) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a scalar value, got %v", node.Kind)
	}
	*s = scalarString(node.Value)
	return nil
}

// stringMap accepts either a map[string]string or a list of "KEY=VALUE"
// strings (used for environment and labels). A bare key with no "=" is
// resolved from the current process environment.
type stringMap map[string]string

func (m *stringMap) UnmarshalYAML(node *yaml.Node) error {
	result := map[string]string{}
	if isNull(node) {
		*m = result
		return nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		var raw map[string]interface{}
		if err := node.Decode(&raw); err != nil {
			return err
		}
		for k, v := range raw {
			if v == nil {
				result[k] = os.Getenv(k)
				continue
			}
			result[k] = fmt.Sprintf("%v", v)
		}
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		for _, item := range list {
			if idx := strings.IndexByte(item, '='); idx >= 0 {
				result[item[:idx]] = item[idx+1:]
			} else {
				result[item] = os.Getenv(item)
			}
		}
	default:
		return fmt.Errorf("expected a map or list of KEY=VALUE strings, got %v", node.Kind)
	}
	*m = result
	return nil
}

type rawPortEntry struct {
	Target    interface{} `yaml:"target"`
	Published interface{} `yaml:"published"`
	Protocol  string      `yaml:"protocol"`
	HostIP    string      `yaml:"host_ip"`
}

// portList accepts short-syntax strings ("8080:80") or long-syntax mappings,
// normalizing everything to strings ready to pass straight to `wslc run -p`.
type portList []string

func (p *portList) UnmarshalYAML(node *yaml.Node) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("ports must be a list")
	}
	var out []string
	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			out = append(out, item.Value)
		case yaml.MappingNode:
			var entry rawPortEntry
			if err := item.Decode(&entry); err != nil {
				return err
			}
			spec := fmt.Sprintf("%v", entry.Published)
			if entry.HostIP != "" {
				spec = entry.HostIP + ":" + spec
			}
			spec += ":" + fmt.Sprintf("%v", entry.Target)
			if entry.Protocol != "" {
				spec += "/" + entry.Protocol
			}
			out = append(out, spec)
		default:
			return fmt.Errorf("invalid ports entry")
		}
	}
	*p = out
	return nil
}

type rawVolumeEntry struct {
	Type     string `yaml:"type"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only"`
}

// volumeEntryList accepts short-syntax strings ("src:dst:ro") or long-syntax
// mappings, normalizing to "src:dst[:ro]" strings for later resolution
// against named volumes.
type volumeEntryList []string

func (v *volumeEntryList) UnmarshalYAML(node *yaml.Node) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("volumes must be a list")
	}
	var out []string
	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			out = append(out, item.Value)
		case yaml.MappingNode:
			var entry rawVolumeEntry
			if err := item.Decode(&entry); err != nil {
				return err
			}
			spec := entry.Source + ":" + entry.Target
			if entry.ReadOnly {
				spec += ":ro"
			}
			out = append(out, spec)
		default:
			return fmt.Errorf("invalid volumes entry")
		}
	}
	*v = out
	return nil
}

// dependsOnMap accepts either a list of service names or a map of
// name -> {condition}.
type dependsOnMap map[string]DependsOn

func (d *dependsOnMap) UnmarshalYAML(node *yaml.Node) error {
	result := map[string]DependsOn{}
	if isNull(node) {
		*d = result
		return nil
	}
	switch node.Kind {
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		for _, name := range list {
			result[name] = DependsOn{}
		}
	case yaml.MappingNode:
		var raw map[string]struct {
			Condition string `yaml:"condition"`
		}
		if err := node.Decode(&raw); err != nil {
			return err
		}
		for name, v := range raw {
			result[name] = DependsOn{Condition: v.Condition}
		}
	default:
		return fmt.Errorf("depends_on must be a list or a map")
	}
	*d = result
	return nil
}

// networkAttachMap accepts either a list of network keys or a map of
// key -> {aliases}.
type networkAttachMap map[string]NetworkAttachment

func (n *networkAttachMap) UnmarshalYAML(node *yaml.Node) error {
	result := map[string]NetworkAttachment{}
	if isNull(node) {
		*n = result
		return nil
	}
	switch node.Kind {
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		for _, name := range list {
			result[name] = NetworkAttachment{}
		}
	case yaml.MappingNode:
		var raw map[string]struct {
			Aliases stringList `yaml:"aliases"`
		}
		if err := node.Decode(&raw); err != nil {
			return err
		}
		for name, v := range raw {
			result[name] = NetworkAttachment{Aliases: v.Aliases}
		}
	default:
		return fmt.Errorf("networks must be a list or a map")
	}
	*n = result
	return nil
}

// ulimitMap accepts `name: value` or `name: {soft, hard}` entries, normalizing
// to "name=value" or "name=soft:hard" strings for `wslc run --ulimit`.
type ulimitMap []string

func (u *ulimitMap) UnmarshalYAML(node *yaml.Node) error {
	if isNull(node) {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("ulimits must be a mapping")
	}
	var out []string
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		valNode := node.Content[i+1]
		switch valNode.Kind {
		case yaml.ScalarNode:
			out = append(out, key+"="+valNode.Value)
		case yaml.MappingNode:
			var lim struct {
				Soft interface{} `yaml:"soft"`
				Hard interface{} `yaml:"hard"`
			}
			if err := valNode.Decode(&lim); err != nil {
				return err
			}
			out = append(out, fmt.Sprintf("%s=%v:%v", key, lim.Soft, lim.Hard))
		default:
			return fmt.Errorf("invalid ulimit entry for %q", key)
		}
	}
	*u = out
	return nil
}

// buildSpec accepts either a shorthand context string or a full mapping.
type buildSpec struct {
	Context    string
	Dockerfile string
	Args       stringMap
	Target     string
}

func (b *buildSpec) UnmarshalYAML(node *yaml.Node) error {
	if isNull(node) {
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		b.Context = node.Value
		return nil
	case yaml.MappingNode:
		var raw struct {
			Context    string    `yaml:"context"`
			Dockerfile string    `yaml:"dockerfile"`
			Args       stringMap `yaml:"args"`
			Target     string    `yaml:"target"`
		}
		if err := node.Decode(&raw); err != nil {
			return err
		}
		b.Context = raw.Context
		b.Dockerfile = raw.Dockerfile
		b.Args = raw.Args
		b.Target = raw.Target
		if b.Context == "" {
			b.Context = "."
		}
		return nil
	}
	return fmt.Errorf("build must be a string or a mapping")
}
