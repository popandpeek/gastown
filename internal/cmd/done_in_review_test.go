package cmd

import (
	"os"
	"strings"
	"testing"
)

// TestDone_SetsInReviewStatus verifies that done.go sets the source issue's
// status to reviewing after successfully creating an MR bead. This is the
// deterministic status transition:
//
//	gt sling → hooked | gt done + MR → reviewing | refinery merge → closed
//
// The status must be set in Go code (not formulas) because agent-dependent
// formula steps can be skipped or fail silently.
func TestDone_SetsInReviewStatus(t *testing.T) {
	// Verify done.go contains the reviewing status update code
	content, err := os.ReadFile("done.go")
	if err != nil {
		t.Fatalf("reading done.go: %v", err)
	}

	src := string(content)

	// Must use the typed StatusInReview constant (not a raw string)
	if !strings.Contains(src, `beads.StatusInReview`) {
		t.Error("done.go must use beads.StatusInReview constant for status update")
	}

	// Must contain the status update via bd.Run with typed constant
	if !strings.Contains(src, `"--status="+string(beads.StatusInReview)`) {
		t.Error("done.go must update issue status to reviewing after MR creation")
	}

	// Must include the comment explaining the deterministic transition
	if !strings.Contains(src, "hooked → reviewing") {
		t.Error("done.go must document the hooked → reviewing status transition")
	}

	// Must be non-fatal (PrintWarning, not Fatalf) — status update failure
	// should not block the merge queue submission
	statusIdx := strings.Index(src, `"--status="+string(beads.StatusInReview)`)
	if statusIdx == -1 {
		t.Fatal("could not find reviewing status update code block")
	}
	// Check the next ~200 chars after the update for PrintWarning
	snippet := src[statusIdx:min(statusIdx+300, len(src))]
	if !strings.Contains(snippet, "PrintWarning") {
		t.Error("reviewing status update must be non-fatal (use PrintWarning, not Fatal)")
	}
}

// TestDone_InReviewAfterMRVerification verifies the reviewing status update
// happens AFTER MR bead verification, not before. Setting reviewing before
// MR verification would leave the bead in reviewing with no MR if verification fails.
func TestDone_InReviewAfterMRVerification(t *testing.T) {
	content, err := os.ReadFile("done.go")
	if err != nil {
		t.Fatalf("reading done.go: %v", err)
	}

	src := string(content)

	mrVerifyIdx := strings.Index(src, "MR bead created but verification read-back failed")
	inReviewIdx := strings.Index(src, `"--status="+string(beads.StatusInReview)`)

	if mrVerifyIdx == -1 {
		t.Fatal("could not find MR verification code")
	}
	if inReviewIdx == -1 {
		t.Fatal("could not find reviewing status update code")
	}

	if inReviewIdx < mrVerifyIdx {
		t.Error("reviewing status update must come AFTER MR bead verification, not before")
	}
}

// TestDone_IsAwaitingMergeGuard verifies that updateAgentStateOnDone skips
// closing beads that are reviewing (awaiting merge). The refinery owns the
// close transition for these beads. See be-ri7ix / hq-65663.
func TestDone_IsAwaitingMergeGuard(t *testing.T) {
	content, err := os.ReadFile("done.go")
	if err != nil {
		t.Fatalf("reading done.go: %v", err)
	}

	src := string(content)

	// The close guard must check IsAwaitingMerge
	if !strings.Contains(src, "IsAwaitingMerge()") {
		t.Error("done.go must check IsAwaitingMerge() before closing hooked bead")
	}

	// The guard must be a negative condition (skip close when awaiting merge)
	if !strings.Contains(src, "!beads.IssueStatus(hookedBead.Status).IsAwaitingMerge()") {
		t.Error("done.go must use !IsAwaitingMerge() guard to skip close for reviewing beads")
	}
}
