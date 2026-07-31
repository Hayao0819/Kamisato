package app

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestRuntimeLoadsOnce(t *testing.T) {
	want := &App{}
	var loads atomic.Int32
	runtime := NewRuntime(func() (*App, error) {
		loads.Add(1)
		return want, nil
	})

	const callers = 16
	var wait sync.WaitGroup
	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			got, err := runtime.App()
			if err != nil || got != want {
				t.Errorf("App() = %p, %v", got, err)
			}
		}()
	}
	wait.Wait()
	if got := loads.Load(); got != 1 {
		t.Fatalf("loader calls = %d, want 1", got)
	}
}

func TestRuntimeRejectsMissingLoaderAndApp(t *testing.T) {
	for name, runtime := range map[string]*Runtime{
		"loader": NewRuntime(nil),
		"app":    NewRuntime(func() (*App, error) { return nil, nil }),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := runtime.App(); err == nil {
				t.Fatal("App() returned no error")
			}
		})
	}
}

func TestLoadedAppDoesNotRunLoader(t *testing.T) {
	var loads atomic.Int32
	runtime := NewRuntime(func() (*App, error) {
		loads.Add(1)
		return &App{}, nil
	})
	if app, loaded := runtime.LoadedApp(); loaded || app != nil {
		t.Fatalf("LoadedApp() = %p, %v", app, loaded)
	}
	if loads.Load() != 0 {
		t.Fatal("LoadedApp ran the loader")
	}
	want, err := runtime.App()
	if err != nil {
		t.Fatal(err)
	}
	if got, loaded := runtime.LoadedApp(); !loaded || got != want {
		t.Fatalf("LoadedApp() = %p, %v; want %p, true", got, loaded, want)
	}
}
