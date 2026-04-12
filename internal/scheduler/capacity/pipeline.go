package capacity

import (
	"sort" // POPANDPEEK-FORK: sp-1 sort helper
	"strings"
)

// PendingBead represents a bead that is scheduled and ready for dispatch evaluation.
type PendingBead struct {
	ID          string             // Context bead ID (sling context)
	WorkBeadID  string             // The actual work bead ID
	Title       string
	TargetRig   string
	Description string
	Labels      []string
	Context     *SlingContextFields // Parsed sling params from context bead

	// POPANDPEEK-FORK BEGIN: sp-1 priority-aware dispatch sort (hq-m5sjf)
	// Populated by getReadySlingContexts from the work bead's bd metadata.
	// Priority: lower = higher-priority (0 = P0). Default 2 (P2) when unset.
	// ReworkRound: 0 = fresh, N = N-th rework bounce. Populated by bp-5.1 once that bead lands.
	// AgeScore: aging boost from sp-3 (Layer 3). sp-1 leaves this at 0.0; sp-3 will populate.
	Priority    int     // POPANDPEEK-FORK: sp-1 priority sort key
	ReworkRound int     // POPANDPEEK-FORK: sp-1 rework tiebreaker, populated by bp-5.1
	AgeScore    float64 // POPANDPEEK-FORK: sp-1 reserved slot for sp-3 aging
	// POPANDPEEK-FORK END
}

// POPANDPEEK-FORK BEGIN: sp-1 SortPending helper (hq-m5sjf)
// SortPending orders a slice of PendingBead by the sp-1 composite dispatch key:
//
//	score        ASC  — float64(Priority) - AgeScore. Lower = higher-priority.
//	ReworkRound  DESC — within score tier, rework rounds have warmer reviewer context.
//	EnqueuedAt   ASC  — within rework tier, FIFO (older first).
//	ID           ASC  — deterministic final tiebreaker.
//
// The floating-point score is the primary key so sp-3's age-amplification can
// modify effective priority (an 8h-old P2 with age_score=2.0 ties a fresh P0
// at score 0.0). sp-1 leaves AgeScore at 0 so score == float64(Priority) and
// behavior is exact integer priority ordering. sp-3 fills in the slot later.
//
// Stable dedup note: this sort happens AFTER the dedup pass in
// getReadySlingContexts (which uses EnqueuedAt ASC to pick the oldest context
// for each work bead). Two contexts for the same work bead are never both in
// the slice passed here, so the sort is safe regardless of dedup semantics.
func SortPending(pending []PendingBead) {
	sort.SliceStable(pending, func(i, j int) bool {
		a, b := pending[i], pending[j]
		scoreA := float64(a.Priority) - a.AgeScore
		scoreB := float64(b.Priority) - b.AgeScore
		if scoreA != scoreB {
			return scoreA < scoreB
		}
		if a.ReworkRound != b.ReworkRound {
			return a.ReworkRound > b.ReworkRound
		}
		aEnq := ""
		bEnq := ""
		if a.Context != nil {
			aEnq = a.Context.EnqueuedAt
		}
		if b.Context != nil {
			bEnq = b.Context.EnqueuedAt
		}
		if aEnq != bEnq {
			return aEnq < bEnq
		}
		return a.ID < b.ID
	})
}

// POPANDPEEK-FORK END

// SlingContextFields holds scheduling parameters stored on a sling context bead.
// JSON-serialized as the context bead's description.
type SlingContextFields struct {
	Version          int    `json:"version"`
	WorkBeadID       string `json:"work_bead_id"`
	TargetRig        string `json:"target_rig"`
	Formula          string `json:"formula,omitempty"`
	Args             string `json:"args,omitempty"`
	Vars             string `json:"vars,omitempty"`
	EnqueuedAt       string `json:"enqueued_at"`
	Merge            string `json:"merge,omitempty"`
	Convoy           string `json:"convoy,omitempty"`
	BaseBranch       string `json:"base_branch,omitempty"`
	NoMerge          bool   `json:"no_merge,omitempty"`
	ReviewOnly       bool   `json:"review_only,omitempty"`
	Account          string `json:"account,omitempty"`
	Agent            string `json:"agent,omitempty"`
	HookRawBead      bool   `json:"hook_raw_bead,omitempty"`
	Owned            bool   `json:"owned,omitempty"`
	Mode             string `json:"mode,omitempty"`
	DispatchFailures int    `json:"dispatch_failures,omitempty"`
	LastFailure      string `json:"last_failure,omitempty"`
}

// LabelSlingContext is the label used to identify sling context beads.
const LabelSlingContext = "gt:sling-context"

// DispatchPlan is the output of PlanDispatch — what to dispatch and why.
type DispatchPlan struct {
	ToDispatch []PendingBead
	Skipped    int
	Reason     string // "capacity" | "batch" | "ready" | "none"
}

// FailureAction indicates what to do after a dispatch failure.
type FailureAction int

const (
	// FailureRetry means the bead should be retried on the next cycle.
	FailureRetry FailureAction = iota
	// FailureQuarantine means the bead should be marked as permanently failed.
	FailureQuarantine
)

// ReadinessFilter is a function that filters pending beads to those ready for dispatch.
type ReadinessFilter func(pending []PendingBead) []PendingBead

// FailurePolicy is a function that determines what to do after N failures.
type FailurePolicy func(failures int) FailureAction

// AllReady is a ReadinessFilter that passes all beads through (no filtering).
func AllReady(pending []PendingBead) []PendingBead {
	return pending
}

// BlockerAware returns a ReadinessFilter that only passes beads whose WorkBeadID
// appears in the readyIDs set (i.e., beads whose work bead has no unresolved blockers).
func BlockerAware(readyIDs map[string]bool) ReadinessFilter {
	return func(pending []PendingBead) []PendingBead {
		var result []PendingBead
		for _, b := range pending {
			if readyIDs[b.WorkBeadID] {
				result = append(result, b)
			}
		}
		return result
	}
}

// PlanDispatch computes which beads to dispatch given capacity constraints.
// availableCapacity: free slots (positive = that many slots, <= 0 = no capacity).
// batchSize: max beads per cycle.
// ready: beads that passed readiness filtering.
func PlanDispatch(availableCapacity, batchSize int, ready []PendingBead) DispatchPlan {
	if len(ready) == 0 {
		return DispatchPlan{Reason: "none"}
	}

	if availableCapacity <= 0 {
		return DispatchPlan{
			Skipped: len(ready),
			Reason:  "capacity",
		}
	}

	// Dispatch up to the smallest of capacity, batchSize, and readyBeads count
	toDispatch := batchSize
	if availableCapacity < toDispatch {
		toDispatch = availableCapacity
	}
	if len(ready) < toDispatch {
		toDispatch = len(ready)
	}

	reason := "batch"
	if availableCapacity < batchSize && availableCapacity < len(ready) {
		reason = "capacity"
	}
	if len(ready) < batchSize && len(ready) < availableCapacity {
		reason = "ready"
	}

	return DispatchPlan{
		ToDispatch: ready[:toDispatch],
		Skipped:    len(ready) - toDispatch,
		Reason:     reason,
	}
}

// NoRetryPolicy returns a FailurePolicy that always quarantines on first failure.
func NoRetryPolicy() FailurePolicy {
	return func(failures int) FailureAction {
		return FailureQuarantine
	}
}

// CircuitBreakerPolicy returns a FailurePolicy that retries up to maxFailures
// times, then quarantines.
func CircuitBreakerPolicy(maxFailures int) FailurePolicy {
	return func(failures int) FailureAction {
		if failures >= maxFailures {
			return FailureQuarantine
		}
		return FailureRetry
	}
}

// FilterCircuitBroken removes beads that have exceeded the maximum dispatch
// failures threshold. Returns the filtered list and the count of removed beads.
func FilterCircuitBroken(beads []PendingBead, maxFailures int) ([]PendingBead, int) {
	var result []PendingBead
	removed := 0
	for _, b := range beads {
		if b.Context != nil && b.Context.DispatchFailures >= maxFailures {
			removed++
			continue
		}
		result = append(result, b)
	}
	return result, removed
}

// DispatchParams captures what the scheduler needs to tell the dispatcher.
// Mirrors the relevant fields from cmd.SlingParams but is scheduler-owned.
type DispatchParams struct {
	BeadID      string
	FormulaName string
	RigName     string
	Args        string
	Vars        []string
	Merge       string
	BaseBranch  string
	Account     string
	Agent       string
	Mode        string
	NoMerge     bool
	ReviewOnly  bool
	HookRawBead bool
}

// ReconstructFromContext builds DispatchParams from sling context fields.
func ReconstructFromContext(ctx *SlingContextFields) DispatchParams {
	p := DispatchParams{
		BeadID:      ctx.WorkBeadID,
		RigName:     ctx.TargetRig,
		FormulaName: ctx.Formula,
		Args:        ctx.Args,
		Merge:       ctx.Merge,
		BaseBranch:  ctx.BaseBranch,
		Account:     ctx.Account,
		Agent:       ctx.Agent,
		Mode:        ctx.Mode,
		NoMerge:     ctx.NoMerge,
		ReviewOnly:  ctx.ReviewOnly,
		HookRawBead: ctx.HookRawBead,
	}
	if ctx.Vars != "" {
		p.Vars = splitVars(ctx.Vars)
	}
	return p
}

// splitVars splits a newline-separated vars string into individual key=value pairs.
func splitVars(vars string) []string {
	if vars == "" {
		return nil
	}
	var result []string
	for _, line := range strings.Split(vars, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
