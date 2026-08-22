package cli

import (
	"fmt"
	"path/filepath"

	"wslc-compose/internal/compose"
	"wslc-compose/internal/wslc"
)

func Up(client wslc.Runner, proj *compose.Project) error {
	for _, key := range sortedKeys(proj.Networks) {
		net := proj.Networks[key]
		if net.External {
			fmt.Printf("network %s: external, skipping create\n", net.Name)
			continue
		}
		labels, exists, _ := wslc.NetworkLabels(client, net.Name)
		if exists {
			if !wslc.Owns(labels, proj.Name) {
				return fmt.Errorf("network %q already exists and is not owned by project %q; refusing to reuse it", net.Name, proj.Name)
			}
			fmt.Printf("network %s: already exists, reusing\n", net.Name)
			continue
		}
		args := []string{"network", "create"}
		if net.Driver != "" {
			args = append(args, "-d", net.Driver)
		}
		for k, v := range net.DriverOpts {
			args = append(args, "-o", k+"="+v)
		}
		for k, v := range net.Labels {
			args = append(args, "-l", k+"="+v)
		}
		args = append(args, "-l", wslc.LabelProject+"="+proj.Name, "-l", wslc.LabelNetwork+"="+key, net.Name)
		fmt.Printf("creating network %s\n", net.Name)
		if err := client.Run(args...); err != nil {
			return fmt.Errorf("creating network %s: %w", net.Name, err)
		}
	}

	for _, key := range sortedKeys(proj.Volumes) {
		vol := proj.Volumes[key]
		if vol.External {
			fmt.Printf("volume %s: external, skipping create\n", vol.Name)
			continue
		}
		labels, exists, _ := wslc.VolumeLabels(client, vol.Name)
		if exists {
			if !wslc.Owns(labels, proj.Name) {
				return fmt.Errorf("volume %q already exists and is not owned by project %q; refusing to reuse it", vol.Name, proj.Name)
			}
			fmt.Printf("volume %s: already exists, reusing\n", vol.Name)
			continue
		}
		args := []string{"volume", "create"}
		if vol.Driver != "" {
			args = append(args, "-d", vol.Driver)
		}
		for k, v := range vol.DriverOpts {
			args = append(args, "-o", k+"="+v)
		}
		for k, v := range vol.Labels {
			args = append(args, "-l", k+"="+v)
		}
		args = append(args, "-l", wslc.LabelProject+"="+proj.Name, "-l", wslc.LabelVolume+"="+key, vol.Name)
		fmt.Printf("creating volume %s\n", vol.Name)
		if err := client.Run(args...); err != nil {
			return fmt.Errorf("creating volume %s: %w", vol.Name, err)
		}
	}

	order, err := proj.StartOrder()
	if err != nil {
		return err
	}

	for _, name := range order {
		svc := proj.Services[name]

		if svc.Build != nil {
			buildArgs := []string{"build", "-t", svc.Image}
			if svc.Build.Dockerfile != "" {
				buildArgs = append(buildArgs, "-f", filepath.Join(svc.Build.Context, svc.Build.Dockerfile))
			}
			if svc.Build.Target != "" {
				buildArgs = append(buildArgs, "--target", svc.Build.Target)
			}
			for k, v := range svc.Build.Args {
				buildArgs = append(buildArgs, "--build-arg", k+"="+v)
			}
			buildArgs = append(buildArgs, svc.Build.Context)
			fmt.Printf("building %s (%s)\n", name, svc.Image)
			if err := client.Run(buildArgs...); err != nil {
				return fmt.Errorf("building service %q: %w", name, err)
			}
		}

		labels, exists, _ := wslc.ContainerLabels(client, svc.ContainerName)
		if exists {
			if !wslc.Owns(labels, proj.Name) {
				return fmt.Errorf("container %q already exists and is not owned by project %q; refusing to reuse it", svc.ContainerName, proj.Name)
			}
			fmt.Printf("service %s: container already exists, starting\n", name)
			if err := client.Run("start", svc.ContainerName); err != nil {
				return fmt.Errorf("starting existing container for service %q: %w", name, err)
			}
			continue
		}

		args := []string{"run", "-d", "--name", svc.ContainerName}
		for _, netKey := range sortedKeys(svc.Networks) {
			net := proj.Networks[netKey]
			attach := svc.Networks[netKey]
			args = append(args, "--network", net.Name)
			for _, alias := range attach.Aliases {
				args = append(args, "--network-alias", alias)
			}
		}
		for _, p := range svc.Ports {
			args = append(args, "-p", p.Raw)
		}
		for _, v := range svc.Volumes {
			args = append(args, "-v", v.Raw)
		}
		for k, v := range svc.Environment {
			args = append(args, "-e", k+"="+v)
		}
		for k, v := range svc.Labels {
			args = append(args, "-l", k+"="+v)
		}
		args = append(args, "-l", wslc.LabelProject+"="+proj.Name, "-l", wslc.LabelService+"="+name)
		if svc.Hostname != "" {
			args = append(args, "-h", svc.Hostname)
		}
		if svc.Domainname != "" {
			args = append(args, "--domainname", svc.Domainname)
		}
		if svc.User != "" {
			args = append(args, "-u", svc.User)
		}
		if svc.WorkingDir != "" {
			args = append(args, "-w", svc.WorkingDir)
		}
		for _, d := range svc.DNS {
			args = append(args, "--dns", d)
		}
		for _, d := range svc.DNSSearch {
			args = append(args, "--dns-search", d)
		}
		if svc.ShmSize != "" {
			args = append(args, "--shm-size", svc.ShmSize)
		}
		if svc.MemLimit != "" {
			args = append(args, "-m", svc.MemLimit)
		}
		if svc.CPUs != "" {
			args = append(args, "--cpus", svc.CPUs)
		}
		for _, t := range svc.Tmpfs {
			args = append(args, "--tmpfs", t)
		}
		if svc.StopSignal != "" {
			args = append(args, "--stop-signal", svc.StopSignal)
		}
		for _, u := range svc.Ulimits {
			args = append(args, "--ulimit", u)
		}

		command := svc.Command
		if len(svc.Entrypoint) > 0 {
			args = append(args, "--entrypoint", svc.Entrypoint[0])
			command = append(append([]string{}, svc.Entrypoint[1:]...), svc.Command...)
		}

		args = append(args, svc.Image)
		args = append(args, command...)

		fmt.Printf("starting service %s (%s)\n", name, svc.ContainerName)
		if err := client.Run(args...); err != nil {
			return fmt.Errorf("running service %q: %w", name, err)
		}
	}

	return nil
}
