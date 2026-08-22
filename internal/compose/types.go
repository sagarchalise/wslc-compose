package compose

type BuildConfig struct {
	Context    string
	Dockerfile string
	Args       map[string]string
	Target     string
}

type PortMapping struct {
	Raw string
}

type VolumeMount struct {
	Raw string
}

type NetworkAttachment struct {
	Aliases []string
}

type DependsOn struct {
	Condition string
}

type Service struct {
	Name          string
	Image         string
	Build         *BuildConfig
	ContainerName string
	Command       []string
	Entrypoint    []string
	Environment   map[string]string
	Ports         []PortMapping
	Volumes       []VolumeMount
	Networks      map[string]NetworkAttachment
	DependsOn     map[string]DependsOn
	Labels        map[string]string
	Hostname      string
	Domainname    string
	User          string
	WorkingDir    string
	DNS           []string
	DNSSearch     []string
	ShmSize       string
	MemLimit      string
	CPUs          string
	Tmpfs         []string
	StopSignal    string
	Ulimits       []string
}

type Network struct {
	Key        string
	Name       string
	Driver     string
	DriverOpts map[string]string
	Labels     map[string]string
	External   bool
}

type Volume struct {
	Key        string
	Name       string
	Driver     string
	DriverOpts map[string]string
	Labels     map[string]string
	External   bool
}

type Project struct {
	Name     string
	Services map[string]Service
	Networks map[string]Network
	Volumes  map[string]Volume
}
