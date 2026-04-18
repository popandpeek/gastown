package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFormulaContent_EmbeddedFormula(t *testing.T) {
	t.Parallel()
	content, err := loadFormulaContent("mol-polecat-work")
	if err != nil {
		t.Fatalf("expected embedded formula to load, got error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected non-empty content from embedded formula")
	}
}

func TestLoadFormulaContent_DiskFallback(t *testing.T) {
	tmpDir := t.TempDir()
	formulasDir := filepath.Join(tmpDir, ".beads", "formulas")
	if err := os.MkdirAll(formulasDir, 0o755); err != nil {
		t.Fatal(err)
	}

	formulaContent := []byte(`formula = "custom-test-formula"
version = 1
description = "test formula on disk"

[[steps]]
id = "step1"
title = "Test step"
description = "A test step"
`)
	if err := os.WriteFile(filepath.Join(formulasDir, "custom-test-formula.formula.toml"), formulaContent, 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	content, err := loadFormulaContent("custom-test-formula")
	if err != nil {
		t.Fatalf("expected disk fallback to load, got error: %v", err)
	}
	if string(content) != string(formulaContent) {
		t.Errorf("content mismatch: got %d bytes, want %d bytes", len(content), len(formulaContent))
	}
}

func TestLoadFormulaContent_NotFound(t *testing.T) {
	t.Parallel()
	_, err := loadFormulaContent("nonexistent-formula-xyz-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent formula")
	}
}

func TestLoadFormulaContent_EmbeddedTakesPrecedence(t *testing.T) {
	t.Parallel()
	content, err := loadFormulaContent("mol-polecat-work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected non-empty content")
	}
}
