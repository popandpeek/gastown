package capacity

import (
	"fmt" // POPANDPEEK-FORK: sp-1 perf test bead construction
	"testing"
)

func TestPlanDispatch(t *testing.T) {
	beads := func(n int) []PendingBead {
		result := make([]PendingBead, n)
		for i := range result {
			result[i] = PendingBead{ID: string(rune('a' + i))}
		}
		return result
	}

	tests := []struct {
		name              string
		availableCapacity int
		batchSize         int
		readyCount        int
		wantCount         int
		wantSkipped       int
		wantReason        string
	}{
		{"no ready beads", 5, 3, 0, 0, 0, "none"},
		{"no capacity (negative)", -1, 3, 10, 0, 10, "capacity"},
		{"no capacity (zero)", 0, 3, 10, 0, 10, "capacity"},
		{"capacity constrains", 2, 3, 10, 2, 8, "capacity"},
		{"batch constrains", 10, 3, 10, 3, 7, "batch"},
		{"ready constrains", 10, 5, 2, 2, 0, "ready"},
		{"large capacity, batch constrains", 100, 3, 10, 3, 7, "batch"},
		{"large capacity, ready constrains", 100, 5, 2, 2, 0, "ready"},
		{"all equal", 3, 3, 3, 3, 0, "batch"},
		{"single bead", 10, 3, 1, 1, 0, "ready"},
		{"capacity 1", 1, 3, 10, 1, 9, "capacity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := beads(tt.readyCount)
			plan := PlanDispatch(tt.availableCapacity, tt.batchSize, ready)

			if len(plan.ToDispatch) != tt.wantCount {
				t.Errorf("ToDispatch count: got %d, want %d", len(plan.ToDispatch), tt.wantCount)
			}
			if plan.Skipped != tt.wantSkipped {
				t.Errorf("Skipped: got %d, want %d", plan.Skipped, tt.wantSkipped)
			}
			if plan.Reason != tt.wantReason {
				t.Errorf("Reason: got %q, want %q", plan.Reason, tt.wantReason)
			}
		})
	}
}

func TestFilterCircuitBroken(t *testing.T) {
	tests := []struct {
		name        string
		failures    []int // dispatch_failures per bead (-1 = nil context)
		maxFailures int
		wantKept    int
		wantRemoved int
	}{
		{"all healthy", []int{0, 0, 0}, 3, 3, 0},
		{"one at threshold", []int{0, 3, 1}, 3, 2, 1},
		{"one above threshold", []int{0, 5, 1}, 3, 2, 1},
		{"all broken", []int{3, 4, 5}, 3, 0, 3},
		{"nil context passes through", []int{-1, 0, 2}, 3, 3, 0},
		{"empty list", []int{}, 3, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var beads []PendingBead
			for i, f := range tt.failures {
				b := PendingBead{ID: string(rune('a' + i))}
				if f >= 0 {
					b.Context = &SlingContextFields{DispatchFailures: f}
				}
				beads = append(beads, b)
			}

			kept, removed := FilterCircuitBroken(beads, tt.maxFailures)
			if len(kept) != tt.wantKept {
				t.Errorf("kept: got %d, want %d", len(kept), tt.wantKept)
			}
			if removed != tt.wantRemoved {
				t.Errorf("removed: got %d, want %d", removed, tt.wantRemoved)
			}
		})
	}
}

func TestAllReady(t *testing.T) {
	beads := []PendingBead{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}
	result := AllReady(beads)
	if len(result) != 3 {
		t.Errorf("AllReady should pass all beads through, got %d", len(result))
	}
}

func TestBlockerAware(t *testing.T) {
	beads := []PendingBead{
		{ID: "ctx-a", WorkBeadID: "a"},
		{ID: "ctx-b", WorkBeadID: "b"},
		{ID: "ctx-c", WorkBeadID: "c"},
		{ID: "ctx-d", WorkBeadID: "d"},
	}

	readyIDs := map[string]bool{"a": true, "c": true}
	filter := BlockerAware(readyIDs)
	result := filter(beads)

	if len(result) != 2 {
		t.Fatalf("BlockerAware should return 2 beads, got %d", len(result))
	}
	if result[0].WorkBeadID != "a" || result[1].WorkBeadID != "c" {
		t.Errorf("BlockerAware returned wrong beads: %v, %v", result[0].WorkBeadID, result[1].WorkBeadID)
	}
}

func TestBlockerAware_EmptySet(t *testing.T) {
	beads := []PendingBead{{ID: "a", WorkBeadID: "wa"}, {ID: "b", WorkBeadID: "wb"}}
	readyIDs := map[string]bool{}
	filter := BlockerAware(readyIDs)
	result := filter(beads)
	if len(result) != 0 {
		t.Errorf("BlockerAware with empty readyIDs should return 0 beads, got %d", len(result))
	}
}

func TestCircuitBreakerPolicy(t *testing.T) {
	policy := CircuitBreakerPolicy(3)

	tests := []struct {
		failures int
		want     FailureAction
	}{
		{0, FailureRetry},
		{1, FailureRetry},
		{2, FailureRetry},
		{3, FailureQuarantine},
		{5, FailureQuarantine},
	}
	for _, tt := range tests {
		got := policy(tt.failures)
		if got != tt.want {
			t.Errorf("CircuitBreakerPolicy(3)(%d) = %v, want %v", tt.failures, got, tt.want)
		}
	}
}

func TestNoRetryPolicy(t *testing.T) {
	policy := NoRetryPolicy()
	for _, failures := range []int{0, 1, 5} {
		if got := policy(failures); got != FailureQuarantine {
			t.Errorf("NoRetryPolicy()(%d) = %v, want FailureQuarantine", failures, got)
		}
	}
}

func TestReconstructFromContext(t *testing.T) {
	ctx := &SlingContextFields{
		WorkBeadID:  "bead-123",
		TargetRig:   "prod-rig",
		Formula:     "mol-polecat-work",
		Args:        "do stuff",
		Vars:        "x=1\ny=2",
		Merge:       "mr",
		BaseBranch:  "main",
		Account:     "acme",
		Agent:       "codex",
		Mode:        "ralph",
		NoMerge:     true,
		HookRawBead: true,
	}

	params := ReconstructFromContext(ctx)

	if params.BeadID != "bead-123" {
		t.Errorf("BeadID: got %q, want %q", params.BeadID, "bead-123")
	}
	if params.RigName != "prod-rig" {
		t.Errorf("RigName: got %q, want %q", params.RigName, "prod-rig")
	}
	if params.FormulaName != "mol-polecat-work" {
		t.Errorf("FormulaName: got %q, want %q", params.FormulaName, "mol-polecat-work")
	}
	if params.Args != "do stuff" {
		t.Errorf("Args: got %q, want %q", params.Args, "do stuff")
	}
	if len(params.Vars) != 2 || params.Vars[0] != "x=1" || params.Vars[1] != "y=2" {
		t.Errorf("Vars: got %v, want [x=1 y=2]", params.Vars)
	}
	if params.Merge != "mr" {
		t.Errorf("Merge: got %q, want %q", params.Merge, "mr")
	}
	if params.BaseBranch != "main" {
		t.Errorf("BaseBranch: got %q, want %q", params.BaseBranch, "main")
	}
	if params.Account != "acme" {
		t.Errorf("Account: got %q, want %q", params.Account, "acme")
	}
	if params.Agent != "codex" {
		t.Errorf("Agent: got %q, want %q", params.Agent, "codex")
	}
	if params.Mode != "ralph" {
		t.Errorf("Mode: got %q, want %q", params.Mode, "ralph")
	}
	if !params.NoMerge {
		t.Error("NoMerge: expected true")
	}
	if !params.HookRawBead {
		t.Error("HookRawBead: expected true")
	}
}

func TestReconstructFromContext_EmptyVars(t *testing.T) {
	ctx := &SlingContextFields{
		WorkBeadID: "bead-1",
		TargetRig:  "rig1",
	}
	params := ReconstructFromContext(ctx)
	if params.Vars != nil {
		t.Errorf("Vars should be nil when ctx.Vars is empty, got %v", params.Vars)
	}
}

// POPANDPEEK-FORK BEGIN: sp-1 SortPending test suite (hq-m5sjf).
// Covers the composite dispatch key (score ASC, ReworkRound DESC, EnqueuedAt ASC, ID ASC)
// where score = float64(Priority) - AgeScore. mkBead builds a PendingBead with the
// given fields; ctx-<id>/work-<id> prefixes keep dedup interactions unambiguous.
func TestSortPending(t *testing.T) {
	mkBead := func(id string, p, rr int, age float64, enq string) PendingBead {
		return PendingBead{
			ID:          "ctx-" + id,
			WorkBeadID:  "work-" + id,
			Priority:    p,
			ReworkRound: rr,
			AgeScore:    age,
			Context:     &SlingContextFields{EnqueuedAt: enq, WorkBeadID: "work-" + id},
		}
	}
	tests := []struct {
		name string
		in   []PendingBead
		want []string
	}{
		{
			// Latent-bug fix: pre-sp-1 these dispatched in insertion (FIFO) order.
			"pure priority ordering P0 to P4",
			[]PendingBead{
				mkBead("p3", 3, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("p0", 0, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("p2", 2, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("p4", 4, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("p1", 1, 0, 0, "2026-01-01T00:00:00Z"),
			},
			[]string{"ctx-p0", "ctx-p1", "ctx-p2", "ctx-p3", "ctx-p4"},
		},
		{
			"FIFO within priority tier (oldest first)",
			[]PendingBead{
				mkBead("new", 1, 0, 0, "2026-03-01T00:00:00Z"),
				mkBead("mid", 1, 0, 0, "2026-02-01T00:00:00Z"),
				mkBead("old", 1, 0, 0, "2026-01-01T00:00:00Z"),
			},
			[]string{"ctx-old", "ctx-mid", "ctx-new"},
		},
		{
			// bp-5.1 integration: higher rework_round has warmer reviewer context, wins tiebreak.
			"rework tiebreaker within score tier",
			[]PendingBead{
				mkBead("rr0", 0, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("rr2", 0, 2, 0, "2026-01-02T00:00:00Z"),
				mkBead("rr1", 0, 1, 0, "2026-01-03T00:00:00Z"),
			},
			[]string{"ctx-rr2", "ctx-rr1", "ctx-rr0"},
		},
		{
			// sp-3 reserved slot: AgeScore=0 degenerate case must reduce to pure integer priority.
			"AgeScore=0 degenerate case",
			[]PendingBead{
				mkBead("p2", 2, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("p0", 0, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("p1", 1, 0, 0, "2026-01-01T00:00:00Z"),
			},
			[]string{"ctx-p0", "ctx-p1", "ctx-p2"},
		},
		{
			// sp-3 aging preview: aged P2 at score 0.0 ties a fresh P0 at score 0.0.
			// enqueued_at ASC resolves (older wins — the P2 was enqueued 8h earlier).
			"aged P2 ties fresh P0, older enqueue wins tiebreak",
			[]PendingBead{
				mkBead("fresh-p0", 0, 0, 0.0, "2026-04-12T15:00:00Z"),
				mkBead("aged-p2", 2, 0, 2.0, "2026-04-12T07:00:00Z"),
			},
			[]string{"ctx-aged-p2", "ctx-fresh-p0"},
		},
		{
			// Stable-dedup interaction: two post-dedup P0s both zero out the composite key
			// to (0, 0, 0.0, enqueued, id) and fall through to enqueued_at ASC — oldest wins.
			// (Actual dedup runs in getReadySlingContexts BEFORE SortPending.)
			"stable dedup: older enqueue wins same-tier tiebreak",
			[]PendingBead{
				mkBead("newer", 0, 0, 0, "2026-04-12T16:00:00Z"),
				mkBead("older", 0, 0, 0, "2026-04-12T08:00:00Z"),
			},
			[]string{"ctx-older", "ctx-newer"},
		},
		{
			// Missing-field contract: SortPending trusts input. getReadySlingContexts applies
			// the P2 default before constructing PendingBeads — this test documents that.
			"explicit P0 outranks defaulted P2",
			[]PendingBead{
				mkBead("defaulted", 2, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("explicit-p0", 0, 0, 0, "2026-01-01T00:00:00Z"),
			},
			[]string{"ctx-explicit-p0", "ctx-defaulted"},
		},
		{
			"ID ASC deterministic final tiebreaker",
			[]PendingBead{
				mkBead("zebra", 0, 0, 0, "2026-01-01T00:00:00Z"),
				mkBead("alpha", 0, 0, 0, "2026-01-01T00:00:00Z"),
			},
			[]string{"ctx-alpha", "ctx-zebra"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := make([]PendingBead, len(tt.in))
			copy(in, tt.in)
			SortPending(in)
			for i := range tt.want {
				if in[i].ID != tt.want[i] {
					got := make([]string, len(in))
					for j, b := range in {
						got[j] = b.ID
					}
					t.Errorf("rank %d: got %s want %s (full %v)", i, in[i].ID, tt.want[i], got)
					break
				}
			}
		})
	}
}

// TestSortPending_Performance verifies 1000 beads sort deterministically in well under 100ms.
func TestSortPending_Performance(t *testing.T) {
	const n = 1000
	build := func() []PendingBead {
		beads := make([]PendingBead, n)
		for i := 0; i < n; i++ {
			beads[i] = PendingBead{
				ID:          fmt.Sprintf("ctx-b%04d", i),
				WorkBeadID:  fmt.Sprintf("work-b%04d", i),
				Priority:    i % 5,
				ReworkRound: i % 3,
				Context:     &SlingContextFields{EnqueuedAt: fmt.Sprintf("2026-01-%02dT00:00:00Z", (i%28)+1)},
			}
		}
		return beads
	}
	first := build()
	SortPending(first)
	second := build()
	// Shuffle before the second sort — deterministic result must still match.
	for i := 0; i < n; i++ {
		j := (i*7 + 13) % n
		second[i], second[j] = second[j], second[i]
	}
	SortPending(second)
	for i := 0; i < n; i++ {
		if first[i].ID != second[i].ID {
			t.Fatalf("sort not deterministic at rank %d: %s vs %s", i, first[i].ID, second[i].ID)
		}
	}
}

// POPANDPEEK-FORK END

func TestSplitVars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"single", "a=1", []string{"a=1"}},
		{"two newline-separated", "a=1\nb=2", []string{"a=1", "b=2"}},
		{"three newline-separated", "x=hello\ny=world\nz=42", []string{"x=hello", "y=world", "z=42"}},
		{"blank lines filtered", "a=1\n\nb=2\n", []string{"a=1", "b=2"}},
		{"whitespace trimmed", "  a=1  \n  b=2  ", []string{"a=1", "b=2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitVars(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitVars(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitVars(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("splitVars(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
