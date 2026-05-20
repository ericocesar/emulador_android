package adb

import (
	"fmt"
	"strings"
)

func (c *ADBClient) InstallAPK(port int, apkPath string) error {
	out, err := c.Exec(port, "install", "-r", "-g", apkPath)
	if err != nil {
		return fmt.Errorf("install APK failed: %w (%s)", err, string(out))
	}
	outStr := string(out)
	if strings.Contains(outStr, "Failure") {
		return fmt.Errorf("install APK failed: %s", outStr)
	}
	return nil
}

func (c *ADBClient) IsPackageInstalled(port int, packageName string) (bool, error) {
	out, err := c.Exec(port, "shell", "pm", "list", "packages", packageName)
	if err != nil {
		return false, fmt.Errorf("check package failed: %w (%s)", err, string(out))
	}
	return strings.Contains(string(out), "package:"+packageName), nil
}
