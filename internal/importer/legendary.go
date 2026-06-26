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
		// Don't auto-install on macOS — the Python toolchain is too fragile.
		// Print a one-liner and let Epic() fall back to EGL manifests.
		fmt.Println("legendary not found. To import your full Epic library, install it manually:")
		fmt.Println("  brew install pipx && pipx install legendary-gl")
		return fmt.Errorf("legendary not installed")
	}

	// Windows: auto-install via winget.
	fmt.Printf("legendary not found. Install via `winget install derrod.legendary`? [y/N] ")
	var resp string
	fmt.Scanln(&resp)
	if strings.ToLower(strings.TrimSpace(resp)) != "y" {
		return fmt.Errorf("aborted")
	}

	cmd := exec.Command("winget", "install", "derrod.legendary")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Ignore exit code — winget exits non-zero for "no upgrade needed".
	_ = cmd.Run()
	return nil
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
