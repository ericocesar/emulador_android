package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsageMB float64 `json:"memory_usage_mb"`
	MemoryLimitMB float64 `json:"memory_limit_mb"`
	Uptime        string  `json:"uptime"`
}

func GetContainerStats(cli *client.Client, containerID string) (*ContainerStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container stats: %w", err)
	}
	defer resp.Body.Close()

	var statsJSON container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&statsJSON); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	// Calculate CPU percentage
	cpuDelta := float64(statsJSON.CPUStats.CPUUsage.TotalUsage - statsJSON.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(statsJSON.CPUStats.SystemUsage - statsJSON.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		onlineCPUs := float64(statsJSON.CPUStats.OnlineCPUs)
		if onlineCPUs == 0 {
			onlineCPUs = float64(len(statsJSON.CPUStats.CPUUsage.PercpuUsage))
		}
		if onlineCPUs == 0 {
			onlineCPUs = 1
		}
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	memUsageMB := float64(statsJSON.MemoryStats.Usage) / 1024 / 1024
	memLimitMB := float64(statsJSON.MemoryStats.Limit) / 1024 / 1024

	// Get uptime from container inspect
	inspectCtx, inspectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer inspectCancel()

	info, err := cli.ContainerInspect(inspectCtx, containerID)
	uptime := "unknown"
	if err == nil && info.State.StartedAt != "" {
		startedAt, parseErr := time.Parse(time.RFC3339Nano, info.State.StartedAt)
		if parseErr == nil {
			uptime = time.Since(startedAt).Truncate(time.Second).String()
		}
	}

	return &ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsageMB: memUsageMB,
		MemoryLimitMB: memLimitMB,
		Uptime:        uptime,
	}, nil
}
