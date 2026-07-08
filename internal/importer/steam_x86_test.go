package importer

import "testing"

func TestProgramFilesX86_PrefersEnvVar(t *testing.T) {
	t.Setenv("PROGRAMFILES(X86)", `D:\Custom\Program Files (x86)`)
	t.Setenv("PROGRAMFILES", `D:\Custom\Program Files`)

	got := programFilesX86()
	want := `D:\Custom\Program Files (x86)`
	if got != want {
		t.Errorf("programFilesX86() = %q, want %q (env var must win when present)", got, want)
	}
}

func TestProgramFilesX86_FallsBackWhenUnset(t *testing.T) {
	t.Setenv("PROGRAMFILES(X86)", "")
	t.Setenv("PROGRAMFILES", `C:\Program Files`)

	got := programFilesX86()
	want := `C:\Program Files (x86)`
	if got != want {
		t.Errorf("programFilesX86() = %q, want %q (must derive from PROGRAMFILES when the parenthesized var is absent)", got, want)
	}
}

func TestProgramFilesX86_EmptyWhenNeitherSet(t *testing.T) {
	t.Setenv("PROGRAMFILES(X86)", "")
	t.Setenv("PROGRAMFILES", "")

	got := programFilesX86()
	if got != "" {
		t.Errorf("programFilesX86() = %q, want empty string when neither var is set", got)
	}
}
