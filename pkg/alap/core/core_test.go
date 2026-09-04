package core

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/capability"
)

func TestAlapAppLifecycle(t *testing.T) {
	app := NewApp("TestApp", "1.0.0").
		SetProfile("os").
		RequireCapability(capability.CapNetwork)

	mounted := false
	running := false
	stopped := false

	app.OnHook(StateMounted, func(a *App) error {
		mounted = true
		return nil
	})

	app.OnHook(StateRunning, func(a *App) error {
		running = true
		return nil
	})

	app.OnHook(StateStopped, func(a *App) error {
		stopped = true
		return nil
	})

	if err := app.Start(); err != nil {
		t.Fatalf("app.Start failed: %v", err)
	}

	if !mounted || !running {
		t.Errorf("expected mounted and running to be true, got mounted=%v running=%v", mounted, running)
	}

	if app.State != StateRunning {
		t.Errorf("expected state RUNNING, got %s", app.State)
	}

	if err := app.Stop(); err != nil {
		t.Fatalf("app.Stop failed: %v", err)
	}

	if !stopped || app.State != StateStopped {
		t.Errorf("expected state STOPPED, got %s", app.State)
	}
}

func TestAppCapabilityEnforcement(t *testing.T) {
	// Web profile requesting Process should fail Start()
	app := NewApp("BadApp", "0.1.0").
		SetProfile("web").
		RequireCapability(capability.CapProcess)

	err := app.Start()
	if err == nil {
		t.Errorf("expected capability failure starting web app with CapProcess, got nil")
	}
}

func TestEventBus(t *testing.T) {
	bus := NewEventBus()
	received := ""

	bus.Subscribe("greet", func(payload interface{}) {
		received = payload.(string)
	})

	bus.Publish("greet", "Hello Alap!")
	if received != "Hello Alap!" {
		t.Errorf("expected 'Hello Alap!', got %q", received)
	}
}
