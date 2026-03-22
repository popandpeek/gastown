package formula

import (
	"strings"
	"testing"
)

// TestPolecatWorkHasVisualSmokeTestStep verifies that the polecat work formula
// includes a visual-smoke-test step that runs browse checks against the dev
// server when src/ or server/ files are changed.
func TestPolecatWorkHasVisualSmokeTestStep(t *testing.T) {
	content, err := formulasFS.ReadFile("formulas/mol-polecat-work.formula.toml")
	if err != nil {
		t.Fatalf("reading polecat work formula: %v", err)
	}

	f, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing polecat work formula: %v", err)
	}

	var step *Step
	for i := range f.Steps {
		if f.Steps[i].ID == "visual-smoke-test" {
			step = &f.Steps[i]
			break
		}
	}

	if step == nil {
		t.Fatal("polecat work formula missing visual-smoke-test step")
	}

	desc := step.Description

	// Must be conditional on src/ or server/ changes
	if !strings.Contains(desc, "src/") || !strings.Contains(desc, "server/") {
		t.Error("visual-smoke-test must check for src/ and server/ file changes")
	}

	// Must use browse binary for navigation
	if !strings.Contains(desc, "goto http://localhost:5173/kanban") {
		t.Error("visual-smoke-test must navigate to /kanban")
	}
	if !strings.Contains(desc, "goto http://localhost:5173/list") {
		t.Error("visual-smoke-test must navigate to /list")
	}

	// Must check console for errors
	if !strings.Contains(desc, "$BROWSE console") {
		t.Error("visual-smoke-test must check browser console for errors")
	}

	// Must verify DOM elements exist via JS
	if !strings.Contains(desc, "$BROWSE js") {
		t.Error("visual-smoke-test must use browse js to verify DOM elements")
	}

	// Must kill dev servers
	if !strings.Contains(desc, "kill $SERVER_PID $VITE_PID") {
		t.Error("visual-smoke-test must kill dev servers after checks")
	}

	// Must start both dev servers
	if !strings.Contains(desc, "npx tsx server/index.ts") {
		t.Error("visual-smoke-test must start the Express backend")
	}
	if !strings.Contains(desc, "npx vite --port 5173") {
		t.Error("visual-smoke-test must start the Vite dev server")
	}
}

// TestPolecatWorkVisualSmokeTestOrdering verifies that visual-smoke-test runs
// after build-check and before commit-changes.
func TestPolecatWorkVisualSmokeTestOrdering(t *testing.T) {
	content, err := formulasFS.ReadFile("formulas/mol-polecat-work.formula.toml")
	if err != nil {
		t.Fatalf("reading polecat work formula: %v", err)
	}

	f, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing polecat work formula: %v", err)
	}

	order, err := f.TopologicalSort()
	if err != nil {
		t.Fatalf("topological sort failed: %v", err)
	}

	indexOf := func(id string) int {
		for i, s := range order {
			if s == id {
				return i
			}
		}
		return -1
	}

	testsIdx := indexOf("run-tests")
	smokeIdx := indexOf("visual-smoke-test")
	commitIdx := indexOf("commit-changes")

	if testsIdx == -1 {
		t.Fatal("run-tests step not found in topological order")
	}
	if smokeIdx == -1 {
		t.Fatal("visual-smoke-test step not found in topological order")
	}
	if commitIdx == -1 {
		t.Fatal("commit-changes step not found in topological order")
	}

	if testsIdx >= smokeIdx {
		t.Errorf("run-tests (idx %d) must come before visual-smoke-test (idx %d)", testsIdx, smokeIdx)
	}
	if smokeIdx >= commitIdx {
		t.Errorf("visual-smoke-test (idx %d) must come before commit-changes (idx %d)", smokeIdx, commitIdx)
	}
}

// TestRefineryPatrolHasVisualSmokeTestStep verifies that the refinery patrol
// formula includes a visual-smoke-test step that runs browse checks against
// the rebased branch and captures screenshots for the quality review record.
func TestRefineryPatrolHasVisualSmokeTestStep(t *testing.T) {
	content, err := formulasFS.ReadFile("formulas/mol-refinery-patrol.formula.toml")
	if err != nil {
		t.Fatalf("reading refinery patrol formula: %v", err)
	}

	f, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing refinery patrol formula: %v", err)
	}

	var step *Step
	for i := range f.Steps {
		if f.Steps[i].ID == "visual-smoke-test" {
			step = &f.Steps[i]
			break
		}
	}

	if step == nil {
		t.Fatal("refinery patrol formula missing visual-smoke-test step")
	}

	desc := step.Description

	// Must be conditional on src/ or server/ changes
	if !strings.Contains(desc, "src/") || !strings.Contains(desc, "server/") {
		t.Error("visual-smoke-test must check for src/ and server/ file changes")
	}

	// Must use browse binary for navigation
	if !strings.Contains(desc, "goto http://localhost:5173/kanban") {
		t.Error("visual-smoke-test must navigate to /kanban")
	}
	if !strings.Contains(desc, "goto http://localhost:5173/list") {
		t.Error("visual-smoke-test must navigate to /list")
	}

	// Must check console for errors
	if !strings.Contains(desc, "$BROWSE console") {
		t.Error("visual-smoke-test must check browser console for errors")
	}

	// Must capture screenshots (refinery-specific requirement)
	if !strings.Contains(desc, "$BROWSE screenshot") {
		t.Error("visual-smoke-test must capture screenshots for quality record")
	}
	if !strings.Contains(desc, "kanban.png") {
		t.Error("visual-smoke-test must screenshot kanban view")
	}
	if !strings.Contains(desc, "list.png") {
		t.Error("visual-smoke-test must screenshot list view")
	}

	// Must verify DOM elements exist via JS
	if !strings.Contains(desc, "$BROWSE js") {
		t.Error("visual-smoke-test must use browse js to verify DOM elements")
	}

	// Must kill dev servers
	if !strings.Contains(desc, "kill $SERVER_PID $VITE_PID") {
		t.Error("visual-smoke-test must kill dev servers after checks")
	}

	// Must operate on the rebased temp branch
	if !strings.Contains(desc, "git checkout temp") {
		t.Error("visual-smoke-test must checkout temp (rebased) branch")
	}
}

// TestRefineryPatrolVisualSmokeTestOrdering verifies that visual-smoke-test
// runs after run-tests and before quality-review in the refinery patrol.
func TestRefineryPatrolVisualSmokeTestOrdering(t *testing.T) {
	content, err := formulasFS.ReadFile("formulas/mol-refinery-patrol.formula.toml")
	if err != nil {
		t.Fatalf("reading refinery patrol formula: %v", err)
	}

	f, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing refinery patrol formula: %v", err)
	}

	order, err := f.TopologicalSort()
	if err != nil {
		t.Fatalf("topological sort failed: %v", err)
	}

	indexOf := func(id string) int {
		for i, s := range order {
			if s == id {
				return i
			}
		}
		return -1
	}

	testsIdx := indexOf("run-tests")
	smokeIdx := indexOf("visual-smoke-test")
	reviewIdx := indexOf("quality-review")

	if testsIdx == -1 {
		t.Fatal("run-tests step not found in topological order")
	}
	if smokeIdx == -1 {
		t.Fatal("visual-smoke-test step not found in topological order")
	}
	if reviewIdx == -1 {
		t.Fatal("quality-review step not found in topological order")
	}

	if testsIdx >= smokeIdx {
		t.Errorf("run-tests (idx %d) must come before visual-smoke-test (idx %d)", testsIdx, smokeIdx)
	}
	if smokeIdx >= reviewIdx {
		t.Errorf("visual-smoke-test (idx %d) must come before quality-review (idx %d)", smokeIdx, reviewIdx)
	}
}

// TestVisualSmokeTestUsesSkipPattern verifies both formulas use the same
// deterministic skip pattern: grep for src/ or server/ changes, exit 0 if none.
func TestVisualSmokeTestUsesSkipPattern(t *testing.T) {
	formulas := []string{
		"formulas/mol-polecat-work.formula.toml",
		"formulas/mol-refinery-patrol.formula.toml",
	}

	for _, path := range formulas {
		content, err := formulasFS.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		f, err := Parse(content)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		var step *Step
		for i := range f.Steps {
			if f.Steps[i].ID == "visual-smoke-test" {
				step = &f.Steps[i]
				break
			}
		}

		if step == nil {
			t.Fatalf("%s missing visual-smoke-test step", path)
		}

		desc := step.Description

		// Must use deterministic grep pattern to decide skip
		if !strings.Contains(desc, `grep -qE '^(src/|server/)'`) {
			t.Errorf("%s: visual-smoke-test must use grep -qE '^(src/|server/)' to detect frontend changes", path)
		}

		// Must skip cleanly (exit 0) when no changes
		if !strings.Contains(desc, "SKIP: No frontend/server changes") {
			t.Errorf("%s: visual-smoke-test must print skip message when no frontend changes", path)
		}
	}
}
