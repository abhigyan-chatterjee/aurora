package pacman

import (
	"os/exec"
	"strings"
)

func GetForeignPackages() (map[string]string, error) {
	cmd := exec.Command("pacman", "-Qm")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return make(map[string]string), nil
		}
		return nil, err
	}

	packages := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return packages, nil
	}

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			packages[parts[0]] = parts[1]
		}
	}
	return packages, nil
}

func IsNewerVersion(localVer, aurVer string) bool {
	if localVer == "" || aurVer == "" || localVer == aurVer {
		return false
	}
	cmd := exec.Command("vercmp", aurVer, localVer)
	output, err := cmd.Output()
	if err != nil {
		return aurVer != localVer
	}
	return strings.TrimSpace(string(output)) == "1"
}
