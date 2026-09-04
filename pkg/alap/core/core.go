package core

import (
	"fmt"
	"sync"

	"github.com/joysriramsarkar/nilLang/pkg/capability"
)

// LifecycleState represents app execution phase
type LifecycleState string

const (
	StateInit     LifecycleState = "INIT"
	StateMounted  LifecycleState = "MOUNTED"
	StateRunning  LifecycleState = "RUNNING"
	StatePaused   LifecycleState = "PAUSED"
	StateStopping LifecycleState = "STOPPING"
	StateStopped  LifecycleState = "STOPPED"
)

// LifecycleHook is a callback executed at state transitions
type LifecycleHook func(app *App) error

// EventBus provides publish-subscribe messaging across application components
type EventBus struct {
	subscribers map[string][]func(payload interface{})
	mu          sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]func(payload interface{})),
	}
}

func (eb *EventBus) Subscribe(event string, handler func(payload interface{})) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.subscribers[event] = append(eb.subscribers[event], handler)
}

func (eb *EventBus) Publish(event string, payload interface{}) {
	eb.mu.RLock()
	handlers := eb.subscribers[event]
	eb.mu.RUnlock()

	for _, h := range handlers {
		h(payload)
	}
}

// App represents an Alap Application Container
type App struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Profile      string                 `json:"profile"`
	Capabilities []capability.Type      `json:"capabilities"`
	State        LifecycleState         `json:"state"`
	Config       map[string]interface{} `json:"config"`
	EventBus     *EventBus              `json:"-"`
	hooks        map[LifecycleState][]LifecycleHook
	mu           sync.RWMutex
}

// NewApp creates a new Alap Application
func NewApp(name, version string) *App {
	return &App{
		Name:         name,
		Version:      version,
		Profile:      "os",
		Capabilities: []capability.Type{capability.CapNetwork, capability.CapFilesystem},
		State:        StateInit,
		Config:       make(map[string]interface{}),
		EventBus:     NewEventBus(),
		hooks:        make(map[LifecycleState][]LifecycleHook),
	}
}

// RequireCapability declares a required capability
func (a *App) RequireCapability(c capability.Type) *App {
	a.Capabilities = append(a.Capabilities, c)
	return a
}

// SetProfile sets the target execution profile
func (a *App) SetProfile(profile string) *App {
	a.Profile = profile
	return a
}

// OnHook registers a lifecycle hook
func (a *App) OnHook(state LifecycleState, hook LifecycleHook) *App {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hooks[state] = append(a.hooks[state], hook)
	return a
}

// Transition moves application to a new lifecycle state, running hooks
func (a *App) Transition(newState LifecycleState) error {
	a.mu.Lock()
	a.State = newState
	hooks := a.hooks[newState]
	a.mu.Unlock()

	for _, h := range hooks {
		if err := h(a); err != nil {
			return fmt.Errorf("lifecycle hook failed on state %s: %w", newState, err)
		}
	}

	a.EventBus.Publish(fmt.Sprintf("app:state:%s", newState), a)
	return nil
}

// Start boots the application through Init -> Mounted -> Running
func (a *App) Start() error {
	// Verify capabilities before starting
	res := capability.VerifyProfileCapabilities(a.Profile, a.Capabilities)
	if !res.Valid {
		return fmt.Errorf("capability verification failed on profile %s: %v", a.Profile, res.Violations)
	}

	if err := a.Transition(StateMounted); err != nil {
		return err
	}
	return a.Transition(StateRunning)
}

// Stop shuts down the application
func (a *App) Stop() error {
	if err := a.Transition(StateStopping); err != nil {
		return err
	}
	return a.Transition(StateStopped)
}
