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

// TestLoadDotEnvFindsAppsEnvFromNestedDir перевіряє, що демо, запущене зі своєї
// теки, саме знаходить спільний apps/.env репозиторію — без явного export.
func TestLoadDotEnvFindsAppsEnvFromNestedDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps"), 0o755); err != nil {
		t.Fatalf("MkdirAll apps: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "demo", "adk-quickstart"), 0o755); err != nil {
		t.Fatalf("MkdirAll demo: %v", err)
	}

	envPath := filepath.Join(root, "apps", ".env")
	if err := os.WriteFile(envPath, []byte("FROM_APPS_ENV=works\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("FROM_APPS_ENV", "")
	_ = os.Unsetenv("FROM_APPS_ENV")

	t.Chdir(filepath.Join(root, "demo", "adk-quickstart"))
	LoadDotEnv()

	if got := os.Getenv("FROM_APPS_ENV"); got != "works" {
		t.Errorf("FROM_APPS_ENV = %q, want %q", got, "works")
	}
}

// TestLoadDotEnvLocalOverridesAppsEnv фіксує порядок пріоритету: локальний .env
// читається першим, тож його значення не перезаписується apps/.env.
func TestLoadDotEnvLocalOverridesAppsEnv(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(filepath.Join(root, "apps"), 0o755); err != nil {
		t.Fatalf("MkdirAll apps: %v", err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("MkdirAll work: %v", err)
	}

	if err := os.WriteFile(filepath.Join(work, ".env"), []byte("PRIORITY=local\n"), 0600); err != nil {
		t.Fatalf("WriteFile local: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", ".env"), []byte("PRIORITY=apps\n"), 0600); err != nil {
		t.Fatalf("WriteFile apps: %v", err)
	}

	t.Setenv("PRIORITY", "")
	_ = os.Unsetenv("PRIORITY")

	t.Chdir(work)
	LoadDotEnv()

	if got := os.Getenv("PRIORITY"); got != "local" {
		t.Errorf("PRIORITY = %q, want %q", got, "local")
	}
}
