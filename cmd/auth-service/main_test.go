package main

import (
	appfx "auth-service/internal/fx"
	"testing"

	"go.uber.org/fx"
)

func TestMain(t *testing.T) {
	t.Helper()

	originalRun := run
	defer func() {
		run = originalRun
	}()

	called := false
	run = func(opts ...fx.Option) {
		called = true
		if len(opts) != 9 {
			t.Fatalf("expected 9 fx modules, got %d", len(opts))
		}
	}

	main()

	if !called {
		t.Fatal("expected run to be called")
	}

	_ = appfx.Module
}
