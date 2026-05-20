package adb

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func (c *ADBClient) Screencap(port int) ([]byte, error) {
	addr := c.Address(port)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "-s", addr, "exec-out", "screencap", "-p")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 255 {
			c.Connect(port)
		}
		return nil, fmt.Errorf("screencap failed: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("screencap returned empty data")
	}
	return out, nil
}

// ScreencapViaDocker captures screen by running screencap -p inside the container
// and piping stdout directly (no file I/O inside Android).
func (c *ADBClient) ScreencapViaDocker(containerName string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "exec", containerName, "screencap", "-p")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker exec screencap failed: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("screencap returned empty data")
	}
	return out, nil
}
