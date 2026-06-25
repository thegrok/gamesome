package importer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type legendaryGame struct {
	AppName  string `json:"app_name"`
	AppTitle string `json:"app_title"`
}


func findLegendary() (string, error) {
	return findLegendaryWithFreshPath(false)
}

func findLegendaryWithFreshPath(freshPath bool) (string, error) {
	if path, err := exec.LookPath("legendary"); err == nil {
		return path, nil
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("legendary not found")
	}
	// winget modifies PATH in the registry but our process inherited the old snapshot.
	// Read machine+user PATH directly from the environment manager via PowerShell.
	if freshPath {
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			`[Environment]::GetEnvironmentVariable('PATH','Machine')+';'+[Environment]::GetEnvironmentVariable('PATH','User')`).Output()
		if err == nil {
			for _, dir := range strings.Split(strings.TrimSpace(string(out)), ";") {
				candidate := filepath.Join(dir, "legendary.exe")
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
		}
	}
	return "", fmt.Errorf("legendary not found")
}

func promptInstallLegendary() error {
	switch runtime.GOOS {
	case "linux":
		return fmt.Errorf("legendary not found on Linux; use 'gamesom import heroic' instead")
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return fmt.Errorf("legendary not found and Homebrew is not installed\nInstall Homebrew first (https://brew.sh), then run: brew install legendary")
		}
	}

	pkgCmd := legendaryPkgCmd()
	fmt.Printf("legendary not found. Install via `%s`? [y/N] ", strings.Join(pkgCmd, " "))
	var resp string
	fmt.Scanln(&resp)
	if strings.ToLower(strings.TrimSpace(resp)) != "y" {
		return fmt.Errorf("aborted")
	}

	cmd := exec.Command(pkgCmd[0], pkgCmd[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Ignore exit code — winget exits non-zero for "no upgrade available"
	// which is not a real failure. findLegendaryWithFreshPath will confirm.
	_ = cmd.Run()
	return nil
}

func legendaryPkgCmd() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"winget", "install", "derrod.legendary"}
	default: // darwin
		return []string{"brew", "install", "legendary"}
	}
}

func legendaryListGames(bin string) ([]byte, error) {
	var stderrBuf strings.Builder
	cmd := exec.Command(bin, "list-games", "--json")
	cmd.Stderr = &stderrBuf
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderrBuf.String(), "No saved credentials") {
			fmt.Println("legendary is not authenticated. Starting auth flow...")
			authCmd := exec.Command(bin, "auth")
			authCmd.Stdin = os.Stdin
			authCmd.Stdout = os.Stdout
			authCmd.Stderr = os.Stderr
			if authErr := authCmd.Run(); authErr != nil {
				return nil, fmt.Errorf("legendary auth failed: %w", authErr)
			}
			// Retry after successful auth
			var retryStderr strings.Builder
			retryCmd := exec.Command(bin, "list-games", "--json")
			retryCmd.Stderr = &retryStderr
			out, err = retryCmd.Output()
			if err != nil {
				fmt.Fprint(os.Stderr, retryStderr.String())
				return nil, fmt.Errorf("legendary list-games failed: %w", err)
			}
			return out, nil
		}
		fmt.Fprint(os.Stderr, stderrBuf.String())
		return nil, fmt.Errorf("legendary list-games failed: %w", err)
	}
	return out, nil
}

