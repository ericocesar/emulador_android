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
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	connectPhaseStart := time.Now()
	log.Printf("WaitForBoot: attempting ADB connect to %s (timeout=%v)", addr, timeout)

	// Phase 1: Connect to ADB daemon
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for ADB connect at %s: %w", addr, ctx.Err())
		case <-ticker.C:
			connectOut, err := exec.CommandContext(ctx, "adb", "connect", addr).CombinedOutput()
			if err != nil {
				elapsed := time.Since(connectPhaseStart).Round(time.Second)
				log.Printf("adb connect %s failed (elapsed=%v): %v (output: %s)", addr, elapsed, err, string(connectOut))
				continue
			}
			outStr := string(connectOut)
			if strings.Contains(outStr, "connected") || strings.Contains(outStr, "already connected") {
				elapsed := time.Since(connectPhaseStart).Round(time.Second)
				log.Printf("adb connected to %s (after %v)", addr, elapsed)
				goto checkBoot
			}
			log.Printf("adb connect %s: %s", addr, outStr)
		}
	}

checkBoot:
	bootPhaseStart := time.Now()
	log.Printf("WaitForBoot: waiting for sys.boot_completed on %s", addr)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for boot_completed at %s: %w", addr, ctx.Err())
		case <-ticker.C:
			out, err := exec.CommandContext(ctx, "adb", "-s", addr, "shell", "getprop", "sys.boot_completed").CombinedOutput()
			if err != nil {
				elapsed := time.Since(bootPhaseStart).Round(time.Second)
				log.Printf("boot_completed check on %s failed (elapsed=%v): %v (output: %s)", addr, elapsed, err, strings.TrimSpace(string(out)))
				continue
			}
			val := strings.TrimSpace(string(out))
			if val == "1" {
				elapsed := time.Since(bootPhaseStart).Round(time.Second)
				log.Printf("device %s boot completed (after %v)", addr, elapsed)
				return nil
			}
			if val != "" {
				log.Printf("device %s boot_completed = %q", addr, val)
			}
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
