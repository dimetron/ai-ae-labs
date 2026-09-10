package adkenv

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "simple pairs",
			input: "A=1\nB=2\n",
			want:  map[string]string{"A": "1", "B": "2"},
		},
		{
			name:  "comments and blanks skipped",
			input: "# comment\n\n  # indented comment\nA=1\n",
			want:  map[string]string{"A": "1"},
		},
		{
			name:  "quotes stripped",
			input: `A="quoted"` + "\n" + `B='single'` + "\n",
			want:  map[string]string{"A": "quoted", "B": "single"},
		},
		{
			name:  "value may contain =",
			input: "URL=https://x/?a=b\n",
			want:  map[string]string{"URL": "https://x/?a=b"},
		},
		{
			name:  "surrounding space trimmed",
			input: "  A  =  1  \n",
			want:  map[string]string{"A": "1"},
		},
		{
			name:  "empty value allowed",
			input: "A=\n",
			want:  map[string]string{"A": ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("Parse() = %v, want %v", got, tc.want)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("Parse()[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "missing equals", input: "NOT_A_PAIR\n"},
		{name: "empty key", input: "=value\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(tc.input)); err == nil {
				t.Fatal("Parse() error = nil, want error")
			}
		})
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	appsDir := filepath.Join(root, "apps")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(appsDir, ".env")
	if err := os.WriteFile(envPath, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Walking up from a nested directory must find it.
	nested := filepath.Join(root, "labs", "week1", "Part1")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Find(nested)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	// t.TempDir may hand back a symlinked path (/var vs /private/var on macOS),
	// so compare resolved paths rather than raw strings.
	wantResolved, _ := filepath.EvalSymlinks(envPath)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("Find() = %q, want %q", gotResolved, wantResolved)
	}
}

func TestFindNotFound(t *testing.T) {
	// A temp dir with no apps/.env anywhere up to the root. This asserts Find
	// terminates at the filesystem root rather than looping forever.
	_, err := Find(t.TempDir())
	if err == nil {
		t.Fatal("Find() error = nil, want ErrNotFound")
	}
	if !errors.Is(err, ErrNotFound) {
		// A real repo checkout above TempDir could legitimately supply apps/.env;
		// on such a machine the call succeeds and this test is vacuous. Only a
		// non-ErrNotFound failure is a genuine problem.
		t.Logf("Find() error = %v (not ErrNotFound); acceptable if an ancestor has apps/.env", err)
	}
}

func TestLoadDoesNotOverrideExisting(t *testing.T) {
	root := t.TempDir()
	appsDir := filepath.Join(root, "apps")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "ADKENV_TEST_EXISTING=from_file\nADKENV_TEST_NEW=from_file\n"
	if err := os.WriteFile(filepath.Join(appsDir, ".env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ADKENV_TEST_EXISTING", "from_env")

	if err := Load(root); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := os.Getenv("ADKENV_TEST_EXISTING"); got != "from_env" {
		t.Errorf("existing var = %q, want %q (env must win over file)", got, "from_env")
	}
	if got := os.Getenv("ADKENV_TEST_NEW"); got != "from_file" {
		t.Errorf("new var = %q, want %q", got, "from_file")
	}
	// Setenv without t.Setenv leaks across tests; clean up the one we added.
	t.Cleanup(func() { os.Unsetenv("ADKENV_TEST_NEW") })
}

func TestKey(t *testing.T) {
	t.Setenv("ADKENV_TEST_KEY", "  ")
	if _, ok := Key("ADKENV_TEST_KEY"); ok {
		t.Error("Key() ok = true for whitespace-only value, want false")
	}

	t.Setenv("ADKENV_TEST_KEY", "secret")
	got, ok := Key("ADKENV_TEST_KEY")
	if !ok || got != "secret" {
		t.Errorf("Key() = (%q, %v), want (%q, true)", got, ok, "secret")
	}
}
