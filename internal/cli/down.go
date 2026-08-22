package cli

import (
	"fmt"
	"os"

	"wslc-compose/internal/compose"
	"wslc-compose/internal/wslc"
)

func Down(client wslc.Runner, proj *compose.Project) error {
	order, err := proj.StopOrder()
	if err != nil {
		return err
	}

	for _, name := range order {
		svc := proj.Services[name]
		labels, exists, _ := wslc.ContainerLabels(client, svc.ContainerName)
		if !exists {
			continue
		}
		if !wslc.Owns(labels, proj.Name) {
			fmt.Printf("skipping %s: not owned by project %q\n", svc.ContainerName, proj.Name)
			continue
		}
		fmt.Printf("stopping %s\n", svc.ContainerName)
		if err := client.Run("stop", svc.ContainerName); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stopping %s: %v\n", svc.ContainerName, err)
		}
		fmt.Printf("removing %s\n", svc.ContainerName)
		if err := client.Run("remove", "-f", svc.ContainerName); err != nil {
			return fmt.Errorf("removing container %s: %w", svc.ContainerName, err)
		}
	}

	for _, key := range sortedKeys(proj.Networks) {
		net := proj.Networks[key]
		if net.External {
			continue
		}
		labels, exists, _ := wslc.NetworkLabels(client, net.Name)
		if !exists {
			continue
		}
		if !wslc.Owns(labels, proj.Name) {
			fmt.Printf("skipping network %s: not owned by project %q\n", net.Name, proj.Name)
			continue
		}
		fmt.Printf("removing network %s\n", net.Name)
		if err := client.Run("network", "remove", net.Name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: removing network %s: %v\n", net.Name, err)
		}
	}

	return nil
}
