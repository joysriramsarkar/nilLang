// Package lifecycle models the mobile app lifecycle state machine (L8.6):
// onCreate/onResume/onPause/onDestroy/onLowMemory plus state save/restore so
// the Nilang bundle can survive process death on all three platforms.
package lifecycle

import "fmt"

// State is one lifecycle stage. The zero value is StateCreated-adjacent:
// StateUnknown forces callers through OnCreate first.
type State int

const (
	StateUnknown State = iota
	StateCreated
	StateResumed
	StatePaused
	StateDestroyed
)

func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateResumed:
		return "resumed"
	case StatePaused:
		return "paused"
	case StateDestroyed:
		return "destroyed"
	default:
		return "unknown"
	}
}

// Event is a lifecycle callback from the host.
type Event int

const (
	EvCreate Event = iota
	EvResume
	EvPause
	EvDestroy
	EvLowMemory
)

// Machine is the lifecycle state machine. It is deliberately small: every
// transition is a table entry, so an illegal transition is a lookup miss,
// not a branch someone forgot.
type Machine struct {
	state State
	// saved holds the last snapshot written by Save (survives restarts).
	saved map[string]string
	log   []string
}

// New returns a machine in StateUnknown.
func New() *Machine { return &Machine{saved: map[string]string{}} }

// State returns the current stage.
func (m *Machine) State() State { return m.state }

// Log returns the transition history (for tests and host debugging).
func (m *Machine) Log() []string { return append([]string(nil), m.log...) }

var transitions = map[State]map[Event]State{
	StateUnknown:   {EvCreate: StateCreated},
	StateCreated:   {EvResume: StateResumed, EvDestroy: StateDestroyed},
	StateResumed:   {EvPause: StatePaused, EvDestroy: StateDestroyed},
	StatePaused:    {EvResume: StateResumed, EvDestroy: StateDestroyed},
	StateDestroyed: {EvCreate: StateCreated},
}

// Fire advances the machine; EvLowMemory never changes state (it only
// notifies the bundle to shed caches).
func (m *Machine) Fire(ev Event) error {
	if ev == EvLowMemory {
		m.log = append(m.log, fmt.Sprintf("lowMemory@%s", m.state))
		return nil
	}
	next, ok := transitions[m.state][ev]
	if !ok {
		return fmt.Errorf("lifecycle: event %d illegal in state %s", ev, m.state)
	}
	m.log = append(m.log, fmt.Sprintf("%s->%s", m.state, next))
	m.state = next
	return nil
}

// Save snapshots key/value UI state for restoration after process death.
func (m *Machine) Save(entries map[string]string) {
	for k, v := range entries {
		m.saved[k] = v
	}
}

// Restore returns the last snapshot (nil when nothing was saved).
func (m *Machine) Restore() map[string]string {
	if len(m.saved) == 0 {
		return nil
	}
	out := make(map[string]string, len(m.saved))
	for k, v := range m.saved {
		out[k] = v
	}
	return out
}
