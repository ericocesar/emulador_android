package adb

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type ADBClient struct {
	Host string
}

func NewADBClient(host string) *ADBClient {
	return &ADBClient{Host: host}
}

func (c *ADBClient) Address(port int) string {
	return fmt.Sprintf("%s:%d", c.Host, port)
}

func (c *ADBClient) WaitForBoot(ctx context.Context, port int, timeout time.Duration) error {
	addr := c.Address(port)
	deadline := time.After(timeout)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// First, try to connect
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for device boot at %s", addr)
		case <-ticker.C:
			connectOut, err := exec.CommandContext(ctx, "adb", "connect", addr).CombinedOutput()
			if err != nil {
				log.Printf("adb connect %s: %v (%s)", addr, err, string(connectOut))
				continue
			}
			outStr := string(connectOut)
			if strings.Contains(outStr, "connected") || strings.Contains(outStr, "already connected") {
				log.Printf("adb connected to %s", addr)
				goto checkBoot
			}
			log.Printf("adb connect %s: %s", addr, outStr)
		}
	}

checkBoot:
	// Now wait for sys.boot_completed
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for boot_completed at %s", addr)
		case <-ticker.C:
			out, err := exec.CommandContext(ctx, "adb", "-s", addr, "shell", "getprop", "sys.boot_completed").CombinedOutput()
			if err != nil {
				log.Printf("waiting for boot_completed on %s: %v", addr, err)
				continue
			}
			if strings.TrimSpace(string(out)) == "1" {
				log.Printf("device %s boot completed", addr)
				return nil
			}
			log.Printf("device %s boot_completed = %q", addr, strings.TrimSpace(string(out)))
		}
	}
}

func (c *ADBClient) Connect(port int) error {
	addr := c.Address(port)
	out, err := exec.Command("adb", "connect", addr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb connect %s failed: %w (%s)", addr, err, string(out))
	}
	outStr := strings.TrimSpace(string(out))
	if strings.Contains(outStr, "connected") || strings.Contains(outStr, "already connected") {
		return nil
	}
	return fmt.Errorf("adb connect %s unexpected: %s", addr, outStr)
}

func (c *ADBClient) Exec(port int, args ...string) ([]byte, error) {
	addr := c.Address(port)
	fullArgs := append([]string{"-s", addr}, args...)
	out, err := exec.Command("adb", fullArgs...).CombinedOutput()
	if err == nil {
		return out, nil
	}
	// Auto-reconnect once if the daemon doesn't know about this device
	// (typical after a gateway restart with a stateless adb daemon).
	if strings.Contains(string(out), "not found") || strings.Contains(string(out), "offline") {
		_ = c.Connect(port)
		out, err = exec.Command("adb", fullArgs...).CombinedOutput()
	}
	return out, err
}
