package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // relative path → content
		expected string
	}{
		{
			name:     "JS project (package.json in mayor/rig)",
			files:    map[string]string{"mayor/rig/package.json": "{}"},
			expected: "js",
		},
		{
			name:     "Go project (go.mod in mayor/rig)",
			files:    map[string]string{"mayor/rig/go.mod": "module test"},
			expected: "go",
		},
		{
			name:     "Rust project (Cargo.toml in mayor/rig)",
			files:    map[string]string{"mayor/rig/Cargo.toml": "[package]"},
			expected: "rust",
		},
		{
			name:     "Python project (pyproject.toml in mayor/rig)",
			files:    map[string]string{"mayor/rig/pyproject.toml": "[project]"},
			expected: "python",
		},
		{
			name:     "JS project in crew clone",
			files:    map[string]string{"crew/alice/package.json": "{}"},
			expected: "js",
		},
		{
			name:     "Unknown project (no markers)",
			files:    map[string]string{"mayor/rig/README.md": "hello"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			townRoot := t.TempDir()
			rigName := "testrig"

			for relPath, content := range tt.files {
				fullPath := filepath.Join(townRoot, rigName, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := detectProjectType(townRoot, rigName)
			if got != tt.expected {
				t.Errorf("detectProjectType() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestWarnMissingTestCommand(t *testing.T) {
	t.Run("returns nil when test_command is set", func(t *testing.T) {
		vars := []string{"test_command=npm run validate", "lint_command=npm run lint"}
		err := warnMissingTestCommand("/tmp", "testrig", vars)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("returns nil when project type is unknown", func(t *testing.T) {
		townRoot := t.TempDir()
		rigName := "testrig"
		// Create rig dir with no project markers
		os.MkdirAll(filepath.Join(townRoot, rigName, "mayor", "rig"), 0755)

		vars := []string{"lint_command=npm run lint"} // no test_command
		err := warnMissingTestCommand(townRoot, rigName, vars)
		if err != nil {
			t.Errorf("expected nil for unknown project, got %v", err)
		}
	})

	t.Run("returns error when JS project has no test_command", func(t *testing.T) {
		townRoot := t.TempDir()
		rigName := "testrig"
		// Create JS project marker
		rigDir := filepath.Join(townRoot, rigName, "mayor", "rig")
		os.MkdirAll(rigDir, 0755)
		os.WriteFile(filepath.Join(rigDir, "package.json"), []byte("{}"), 0644)

		vars := []string{"lint_command=npm run lint"} // no test_command
		err := warnMissingTestCommand(townRoot, rigName, vars)
		if err == nil {
			t.Error("expected error for JS project with no test_command")
		}
	})

	t.Run("returns error when test_command is empty string", func(t *testing.T) {
		townRoot := t.TempDir()
		rigName := "testrig"
		rigDir := filepath.Join(townRoot, rigName, "mayor", "rig")
		os.MkdirAll(rigDir, 0755)
		os.WriteFile(filepath.Join(rigDir, "go.mod"), []byte("module test"), 0644)

		vars := []string{"test_command="} // explicitly empty
		err := warnMissingTestCommand(townRoot, rigName, vars)
		if err == nil {
			t.Error("expected error for Go project with empty test_command")
		}
	})
}
