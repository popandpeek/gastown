package cmd

import (
	"os"
	"strings"
	"testing"
)

// TestDone_SetsInReviewStatus verifies that done.go sets the source issue's
// status to in_review after successfully creating an MR bead. This is the
// deterministic status transition:
//   gt sling → hooked | gt done + MR → in_review | refinery merge → closed
//
// The status must be set in Go code (not formulas) because agent-dependent
// formula steps can be skipped or fail silently.
func TestDone_SetsInReviewStatus(t *testing.T) {
	// Verify done.go contains the in_review status update code
	content, err := os.ReadFile("done.go")
	if err != nil {
		t.Fatalf("reading done.go: %v", err)
	}

	src := string(content)

	// Must contain the status update command
	if !strings.Contains(src, `"--status="+inReview`) && !strings.Contains(src, `"--status=in_review"`) {
		t.Error("done.go must update issue status to in_review after MR creation")
	}

	// Must be scoped to when issueID is non-empty
	if !strings.Contains(src, `inReview := "in_review"`) {
		t.Error("done.go must define inReview status variable")
	}

	// Must include the comment explaining the deterministic transition
	if !strings.Contains(src, "hooked → in_review") {
		t.Error("done.go must document the hooked → in_review status transition")
	}

	// Must be non-fatal (PrintWarning, not Fatalf) — status update failure
	// should not block the merge queue submission
	inReviewIdx := strings.Index(src, `inReview := "in_review"`)
	if inReviewIdx == -1 {
		t.Fatal("could not find in_review code block")
	}
	// Check the next ~200 chars after the variable for PrintWarning
	snippet := src[inReviewIdx : min(inReviewIdx+300, len(src))]
	if !strings.Contains(snippet, "PrintWarning") {
		t.Error("in_review status update must be non-fatal (use PrintWarning, not Fatal)")
	}
}

// TestDone_InReviewAfterMRVerification verifies the in_review status update
// happens AFTER MR bead verification, not before. Setting in_review before
// MR verification would leave the bead in in_review with no MR if verification fails.
func TestDone_InReviewAfterMRVerification(t *testing.T) {
	content, err := os.ReadFile("done.go")
	if err != nil {
		t.Fatalf("reading done.go: %v", err)
	}

	src := string(content)

	mrVerifyIdx := strings.Index(src, "MR bead created but verification read-back failed")
	inReviewIdx := strings.Index(src, `inReview := "in_review"`)

	if mrVerifyIdx == -1 {
		t.Fatal("could not find MR verification code")
	}
	if inReviewIdx == -1 {
		t.Fatal("could not find in_review code")
	}

	if inReviewIdx < mrVerifyIdx {
		t.Error("in_review status update must come AFTER MR bead verification, not before")
	}
}
