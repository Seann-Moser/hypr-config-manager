package configfinder

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Seann-Moser/hypr-config-manager/pkg/utils"
)

//go:embed blacklist.txt
var blacklist string

// ConfigFinder struct contains the logic to find config files.
type ConfigFinder struct {
	HomeDir      string
	blacklistReg []*regexp.Regexp
	timeout      int
}

// NewConfigFinder creates a new instance of ConfigFinder.
func NewConfigFinder() (*ConfigFinder, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to get home directory: %v", err)
	}
	var blacklistReg []*regexp.Regexp
	for _, r := range strings.Split(blacklist, "\n") {
		re, err := regexp.Compile(r)
		if err != nil {
			return nil, err
		}
		blacklistReg = append(blacklistReg, re)

	}
	return &ConfigFinder{
		HomeDir:      homeDir,
		blacklistReg: blacklistReg,
		timeout:      2,
	}, nil
}

// SearchCommonLocations searches common directories for config files.
func (cf *ConfigFinder) SearchCommonLocations(program string) []string {
	locations := []string{
		filepath.Join(cf.HomeDir, ".config", program),
		filepath.Join(cf.HomeDir, ".local", "share", program),
		filepath.Join("/etc", program),
		filepath.Join("/usr/share", program),
	}

	var configFiles []string
	for _, location := range locations {
		files, err := findConfigFiles(location)
		if err != nil {
			continue
		}
		configFiles = append(configFiles, files...)
	}

	return configFiles
}

// findConfigFiles searches the given directory for any file named "config", "settings", etc.
func findConfigFiles(dir string) ([]string, error) {
	var configFiles []string
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			// Recursively check subdirectories
			subDir := filepath.Join(dir, file.Name())
			subFiles, err := findConfigFiles(subDir)
			if err != nil {
				continue
			}
			configFiles = append(configFiles, subFiles...)
		} else {
			if strings.Contains(file.Name(), "config") || strings.Contains(file.Name(), "settings") {
				configFiles = append(configFiles, filepath.Join(dir, file.Name()))
			}
		}
	}
	return configFiles, nil
}

// RunStrace runs `strace` on the given application to find files it accesses.
func FindPIDByName(programName string) (string, error) {
	cmd := exec.Command("pgrep", "-x", programName)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Run the command
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to find PID for %s: %v", programName, err)
	}

	// Get the PID(s)
	pid := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(pid) == 0 {
		return "", fmt.Errorf("no running process found for %s", programName)
	}

	return pid[0], nil
}

// RunStrace runs `strace` on the given application to find files it accesses.
// RunStrace runs `strace` on the given application to find files it accesses.
// RunStrace runs `strace` on the given application to find files it accesses.
func (cf *ConfigFinder) RunStrace(application string) ([]string, error) {
	// Set a timeout (e.g., 5 seconds)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cf.timeout)*time.Second)
	defer cancel()

	logFile := "/tmp/application.log"

	// Removed `timeout` from the command. Go's context handles timeout/cancellation
	// more reliably with the Process Group fix. The `-k` flag to strace
	// will ensure child processes are traced.
	cmd := exec.CommandContext(ctx, "strace", "-e", "trace=file", "-f", "-o", logFile, application)

	// 💡 CRITICAL FIX 1: Set the process group ID.
	// This makes the OS put `strace` (and the application) into a new process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// --- CRITICAL FIX 2: Process Group Cleanup ---
	// Start the command first.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Get the process group ID (which is the PID of the strace process itself).
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		// If we can't get the PGID, we fall back to killing the single process.
		defer cmd.Process.Kill()
	} else {
		// Defer a function to clean up the entire process group.
		defer func() {
			// Signal the entire process group (negative PID) with SIGKILL.
			// This is the most reliable way to stop strace and all its children.
			// We use SIGKILL (9) to ensure they stop immediately.
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}()
	}
	go func() {
		t := time.NewTimer(time.Duration(cf.timeout) * time.Second)
		select {
		case <-t.C:
			err = exec.Command("kill", "-9", strconv.Itoa(cmd.Process.Pid)).Run()
			if err != nil {
				slog.Info("strace timed out but cleanup should have already been handled.", "e", err)
			}
			err = exec.Command("kill", "-TERM", "-g", strconv.Itoa(-pgid)).Run()
			if err != nil {
				slog.Info("strace timed out but cleanup should have already been handled.", "e", err)
			}
		}
	}()
	// --- End of CRITICAL FIX 2 ---

	// Wait for the command to finish or the context to be canceled.
	err = cmd.Wait()

	// Since we are using an aggressive cleanup (SIGKILL on defer), the process group
	// should be gone. Now, check the error.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// This is the expected timeout case. We consider this a successful run
		// that was simply cut short by the timeout.
		slog.Info("strace timed out but cleanup should have already been handled.")
	} else if err != nil {
		// Handle other non-timeout errors (e.g., command not found, unexpected exit).
		// Since we defer-killed the group, the Wait() may return an error like
		// "signal: killed", which we should ignore if it's due to our cleanup.
		if !strings.Contains(err.Error(), "signal: killed") {
			// Only return a failure if it's a *real* error, not a side-effect of SIGKILL.
			return nil, fmt.Errorf("command failed with error: %w. Output: %s", err, out.String())
		}
	}

	// ... rest of the file parsing and cleanup ...
	// (Your parsing logic here)
	// ...

	// Parse the output to extract the file paths
	var filePaths []string
	data, err := os.ReadFile(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file %s: %w", logFile, err)
	}

	// ... (rest of parsing logic) ...
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, ".config") && strings.Contains(line, "newfstatat") {
			l, err := ExtractBetweenQuotes(line)
			if err != nil {
				continue
			}

			if !cf.isBlacklisted(l) {
				continue
			}

			filePaths = append(filePaths, l)
		}
	}

	// Remove the log file (cleanup from the original function)
	if err := os.Remove(logFile); err != nil {
		slog.Error("failed to remove log file", "file", logFile, "err", err)
	}

	return utils.DeduplicateStrings(filePaths), nil
}
func (cf *ConfigFinder) isBlacklisted(v string) bool {
	for _, r := range cf.blacklistReg {
		if r.MatchString(v) {
			return false
		}
	}
	return true
}

func ExtractBetweenQuotes(input string) (string, error) {
	// Regular expression to match content between quotes
	re := regexp.MustCompile(`"([^"]*)"`)

	// Find the first match
	match := re.FindStringSubmatch(input)

	if len(match) < 2 {
		return "", fmt.Errorf("no content found between quotes")
	}

	// Return the content between the quotes
	return match[1], nil
}

// FindConfigFiles combines all methods to locate configuration files for a program.
func (cf *ConfigFinder) FindConfigFiles(program string) ([]string, error) {
	// Step 1: Search common locations
	commonConfigs := cf.SearchCommonLocations(program)

	// Step 2: Run `strace` to find files accessed by the program
	straceConfigs, err := cf.RunStrace(program)
	if err != nil {
		return nil, err
	}

	// Combine the results
	allConfigs := append(commonConfigs, straceConfigs...)
	return allConfigs, nil
}

// IsStraceInstalled checks if strace is installed on the system.
func (cf *ConfigFinder) IsStraceInstalled() bool {
	cmd := exec.Command("which", "strace")
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}
