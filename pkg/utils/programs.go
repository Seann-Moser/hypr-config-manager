package utils

import (
	"os/exec"
	"strings"
)

// IsProgramInstalled checks if a program is installed on the system
func IsProgramInstalled(program string) bool {
	// Run the `which` command to see if the program is in the system's PATH
	cmd := exec.Command("which", program)
	output, err := cmd.CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		return false
	}
	return true
}

// VerifyPrograms takes a list of program names and returns a map of program names with their installation status
func VerifyPrograms(programs []string) map[string]bool {
	installationStatus := make(map[string]bool)
	for _, program := range programs {
		// Check if the program is installed
		installationStatus[program] = IsProgramInstalled(program)
	}
	return installationStatus
}
