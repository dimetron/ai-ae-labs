package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	content := `
# Коментар
FOO=bar
BAZ="qux"
QUOTED='single'
ALREADY_SET=new_value
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("ALREADY_SET", "original_value")

	LoadDotEnv(envPath)

	if got := os.Getenv("FOO"); got != "bar" {
		t.Errorf("FOO = %q, want %q", got, "bar")
	}
	if got := os.Getenv("BAZ"); got != "qux" {
		t.Errorf("BAZ = %q, want %q", got, "qux")
	}
	if got := os.Getenv("QUOTED"); got != "single" {
		t.Errorf("QUOTED = %q, want %q", got, "single")
	}
	if got := os.Getenv("ALREADY_SET"); got != "original_value" {
		t.Errorf("ALREADY_SET = %q, want %q", got, "original_value")
	}
}

func TestLoadDotEnvNonExistent(t *testing.T) {
	// Не повинно панікувати на неіснуючих файлах
	LoadDotEnv("/non/existent/path/.env")
}
