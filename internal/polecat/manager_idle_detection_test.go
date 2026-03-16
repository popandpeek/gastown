package polecat

import (
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestIdleDetection_ExitTypeRequired verifies that the idle detection logic
// requires agent_state=(idle|done) AND exit_type present to classify a polecat
// as idle-eligible for reuse. Both "idle" and "done" are accepted because
// gt done sets agent_state=idle directly, but if the state update races or
// partially fails, the agent may show "done" instead.
func TestIdleDetection_ExitTypeRequired(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		exitType  string
		wantIdle  bool
	}{
		{
			name:     "idle with exit_type COMPLETED → idle",
			state:    string(beads.AgentStateIdle),
			exitType: "COMPLETED",
			wantIdle: true,
		},
		{
			name:     "idle with exit_type ESCALATED → idle",
			state:    string(beads.AgentStateIdle),
			exitType: "ESCALATED",
			wantIdle: true,
		},
		{
			name:     "idle with exit_type DEFERRED → idle",
			state:    string(beads.AgentStateIdle),
			exitType: "DEFERRED",
			wantIdle: true,
		},
		{
			name:     "done with exit_type COMPLETED → idle (done is idle-eligible)",
			state:    string(beads.AgentStateDone),
			exitType: "COMPLETED",
			wantIdle: true,
		},
		{
			name:     "done with exit_type ESCALATED → idle (done is idle-eligible)",
			state:    string(beads.AgentStateDone),
			exitType: "ESCALATED",
			wantIdle: true,
		},
		{
			name:     "idle WITHOUT exit_type → not idle (could be crashed)",
			state:    string(beads.AgentStateIdle),
			exitType: "",
			wantIdle: false,
		},
		{
			name:     "done WITHOUT exit_type → not idle (could be crashed)",
			state:    string(beads.AgentStateDone),
			exitType: "",
			wantIdle: false,
		},
		{
			name:     "working state with exit_type → not idle",
			state:    string(beads.AgentStateWorking),
			exitType: "COMPLETED",
			wantIdle: false,
		},
		{
			name:     "spawning state with exit_type → not idle",
			state:    string(beads.AgentStateSpawning),
			exitType: "COMPLETED",
			wantIdle: false,
		},
		{
			name:     "empty state → not idle",
			state:    "",
			exitType: "",
			wantIdle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := &beads.AgentFields{
				AgentState: tt.state,
				ExitType:   tt.exitType,
			}
			// This matches the logic in manager.go loadFromBeads:
			// (agent_state=idle OR agent_state=done) AND exit_type present → idle
			agentState := beads.AgentState(fields.AgentState)
			isIdle := (agentState == beads.AgentStateIdle || agentState == beads.AgentStateDone) &&
				fields.ExitType != ""

			if isIdle != tt.wantIdle {
				t.Errorf("idle detection = %v, want %v (state=%q, exitType=%q)",
					isIdle, tt.wantIdle, tt.state, tt.exitType)
			}
		})
	}
}

// TestIdleDetection_NilFields verifies that nil AgentFields don't cause a panic
// in the idle detection logic path.
func TestIdleDetection_NilFields(t *testing.T) {
	var fields *beads.AgentFields
	// Matching the guard in loadFromBeads: agentErr == nil && fields != nil && ...
	isIdle := fields != nil &&
		(beads.AgentState(fields.AgentState) == beads.AgentStateIdle ||
			beads.AgentState(fields.AgentState) == beads.AgentStateDone) &&
		fields.ExitType != ""
	if isIdle {
		t.Error("nil fields should not be detected as idle")
	}
}
