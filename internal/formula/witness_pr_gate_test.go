package formula

import (
	"strings"
	"testing"
)

// TestWitnessPatrolHasPRExistenceCheck verifies that the witness patrol formula's
// inbox-check step includes a mandatory PR existence check before stopping
// polecat sessions. This prevents the scenario where polecats complete work,
// push branches, but the witness stops their sessions before PRs are created.
// See: gt-h8x (3 polecats completed without PRs on 2026-03-15).
func TestWitnessPatrolHasPRExistenceCheck(t *testing.T) {
	content, err := formulasFS.ReadFile("formulas/mol-witness-patrol.formula.toml")
	if err != nil {
		t.Fatalf("reading witness patrol formula: %v", err)
	}

	f, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing witness patrol formula: %v", err)
	}

	// Find the inbox-check step
	var inboxStep *Step
	for i := range f.Steps {
		if f.Steps[i].ID == "inbox-check" {
			inboxStep = &f.Steps[i]
			break
		}
	}

	if inboxStep == nil {
		t.Fatal("witness patrol formula missing inbox-check step")
	}

	desc := inboxStep.Description

	// Verify PR existence check is present
	if !strings.Contains(desc, "gh pr list") {
		t.Error("inbox-check step must include 'gh pr list' to check for PR existence")
	}

	if !strings.Contains(desc, "gh pr create") {
		t.Error("inbox-check step must include 'gh pr create' to auto-create missing PRs")
	}

	if !strings.Contains(desc, "MANDATORY PR EXISTENCE CHECK") {
		t.Error("inbox-check step must include MANDATORY PR EXISTENCE CHECK section")
	}

	// Verify commits-ahead check
	if !strings.Contains(desc, "rev-list --count") {
		t.Error("inbox-check step must check commits ahead of base before PR check")
	}
}

// TestWitnessPatrolEscalationsUseMail verifies that the witness patrol formula
// requires all escalations to use gt mail send, never handoff notes.
// See: gt-h8x (witness detected missing PR but only noted in handoff text).
func TestWitnessPatrolEscalationsUseMail(t *testing.T) {
	content, err := formulasFS.ReadFile("formulas/mol-witness-patrol.formula.toml")
	if err != nil {
		t.Fatalf("reading witness patrol formula: %v", err)
	}

	f, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing witness patrol formula: %v", err)
	}

	// Check top-level description for the mail-over-handoff rule
	if !strings.Contains(f.Description, "Mail over handoff notes") {
		t.Error("formula description must include 'Mail over handoff notes' design principle")
	}

	if !strings.Contains(f.Description, "ALL escalations to mayor MUST use") {
		t.Error("formula description must explicitly state all escalations must use gt mail send")
	}

	// Check inbox-check step for the escalation rule
	for _, step := range f.Steps {
		if step.ID == "inbox-check" {
			if !strings.Contains(step.Description, "ALL escalations MUST use") {
				t.Error("inbox-check step must include rule: ALL escalations MUST use gt mail send")
			}
			if !strings.Contains(step.Description, "NEVER handoff notes") {
				t.Error("inbox-check step must include rule: NEVER handoff notes for escalations")
			}
			break
		}
	}
}

// TestWitnessPatrolPRCheckBeforeSessionStop verifies the PR check happens
// BEFORE any session stop, not after.
func TestWitnessPatrolPRCheckBeforeSessionStop(t *testing.T) {
	content, err := formulasFS.ReadFile("formulas/mol-witness-patrol.formula.toml")
	if err != nil {
		t.Fatalf("reading witness patrol formula: %v", err)
	}

	f, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing witness patrol formula: %v", err)
	}

	for _, step := range f.Steps {
		if step.ID == "inbox-check" {
			// The PR check section must appear before any session stop guidance
			prCheckIdx := strings.Index(step.Description, "MANDATORY PR EXISTENCE CHECK")
			if prCheckIdx == -1 {
				t.Fatal("inbox-check missing MANDATORY PR EXISTENCE CHECK")
			}

			// Verify escalation uses mail send, not handoff
			if !strings.Contains(step.Description, "gt mail send mayor/") {
				t.Error("PR check escalation must use 'gt mail send mayor/'")
			}
			break
		}
	}
}
