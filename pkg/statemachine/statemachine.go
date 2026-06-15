package statemachine

import "fmt"

type Transition struct {
	From   string
	Action string
	To     string
}

type StateMachine struct {
	transitions map[string]map[string]string
}

func New() *StateMachine {
	return &StateMachine{transitions: make(map[string]map[string]string)}
}

func (sm *StateMachine) Register(from, action, to string) {
	if sm.transitions[from] == nil {
		sm.transitions[from] = make(map[string]string)
	}
	sm.transitions[from][action] = to
}

func (sm *StateMachine) Transition(currentStatus, action string) (string, error) {
	actions, ok := sm.transitions[currentStatus]
	if !ok {
		return "", fmt.Errorf("状态 %s 不存在", currentStatus)
	}
	newStatus, ok := actions[action]
	if !ok {
		return "", fmt.Errorf("状态 %s 不允许操作 %s", currentStatus, action)
	}
	return newStatus, nil
}

func DefaultApprovalSM() *StateMachine {
	sm := New()
	sm.Register("draft", "submit", "pending")
	sm.Register("pending", "approve", "approved")
	sm.Register("pending", "reject", "rejected")
	sm.Register("pending", "return", "returned")
	sm.Register("returned", "submit", "pending")
	return sm
}
