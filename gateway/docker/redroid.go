package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func CreateRedroidContainer(cli *client.Client, deviceID string, adbPort int, mem int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if mem <= 0 {
		mem = 1024 * 1024 * 1024 // 1GB
	}

	containerName := fmt.Sprintf("emulador_redroid_%s", deviceID[:8])
	portStr := fmt.Sprintf("%d", adbPort)

	hostConfig := &container.HostConfig{
		Privileged: true,
		PortBindings: nat.PortMap{
			"5555/tcp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: portStr},
			},
		},
		Resources: container.Resources{
			Memory:   mem,
			NanoCPUs: 1_500_000_000, // 1.5 CPU cores limit
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	config := &container.Config{
		Image: "redroid/redroid:11.0.0-latest",
		Cmd: []string{
			"androidboot.redroid_width=480",
			"androidboot.redroid_height=854",
			"androidboot.redroid_dpi=160",
			"androidboot.redroid_fps=10",
			"androidboot.redroid_gpu_mode=guest",
		},
		ExposedPorts: nat.PortSet{
			"5555/tcp": struct{}{},
		},
	}

	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

func StartContainer(cli *client.Client, containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

func StopContainer(cli *client.Client, containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	timeout := 10
	stopOpts := container.StopOptions{Timeout: &timeout}
	return cli.ContainerStop(ctx, containerID, stopOpts)
}

func RemoveContainer(cli *client.Client, containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

func ContainerExists(cli *client.Client, containerID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func IsContainerRunning(cli *client.Client, containerID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return info.State.Running, nil
}
