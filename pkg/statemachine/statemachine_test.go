package statemachine

import "testing"

func TestDefaultApprovalSM(t *testing.T) {
	sm := DefaultApprovalSM()

	tests := []struct {
		name    string
		from    string
		action  string
		want    string
		wantErr bool
	}{
		{"draft to pending", "draft", "submit", "pending", false},
		{"pending approve", "pending", "approve", "approved", false},
		{"pending reject", "pending", "reject", "rejected", false},
		{"pending return", "pending", "return", "returned", false},
		{"returned resubmit", "returned", "submit", "pending", false},
		{"approved no action", "approved", "approve", "", true},
		{"draft no approve", "draft", "approve", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sm.Transition(tt.from, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Transition() = %v, want %v", got, tt.want)
			}
		})
	}
}
