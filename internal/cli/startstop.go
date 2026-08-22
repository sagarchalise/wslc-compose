package cli

import (
	"fmt"

	"wslc-compose/internal/compose"
	"wslc-compose/internal/wslc"
)

func Start(client wslc.Runner, proj *compose.Project) error {
	order, err := proj.StartOrder()
	if err != nil {
		return err
	}
	for _, name := range order {
		svc := proj.Services[name]
		labels, exists, _ := wslc.ContainerLabels(client, svc.ContainerName)
		if !exists {
			fmt.Printf("skipping %s: no such container (run 'wslc-compose up' first)\n", svc.ContainerName)
			continue
		}
		if !wslc.Owns(labels, proj.Name) {
			fmt.Printf("skipping %s: not owned by project %q\n", svc.ContainerName, proj.Name)
			continue
		}
		fmt.Printf("starting %s\n", svc.ContainerName)
		if err := client.Run("start", svc.ContainerName); err != nil {
			return fmt.Errorf("starting %s: %w", svc.ContainerName, err)
		}
	}
	return nil
}

func Stop(client wslc.Runner, proj *compose.Project) error {
	order, err := proj.StopOrder()
	if err != nil {
		return err
	}
	for _, name := range order {
		svc := proj.Services[name]
		labels, exists, _ := wslc.ContainerLabels(client, svc.ContainerName)
		if !exists {
			fmt.Printf("skipping %s: no such container\n", svc.ContainerName)
			continue
		}
		if !wslc.Owns(labels, proj.Name) {
			fmt.Printf("skipping %s: not owned by project %q\n", svc.ContainerName, proj.Name)
			continue
		}
		fmt.Printf("stopping %s\n", svc.ContainerName)
		if err := client.Run("stop", svc.ContainerName); err != nil {
			return fmt.Errorf("stopping %s: %w", svc.ContainerName, err)
		}
	}
	return nil
}
