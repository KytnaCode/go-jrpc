package jrpc_test

import (
	"sync"
	"testing"

	"github.com/kytnacode/go-jrpc"
)

func TestRegistry_RegisterShouldReturnErrorWithInvalidHandler(t *testing.T) {
	t.Parallel()

	type data struct {
		handler any
	}

	testData := map[string]data{
		"without args and reply": {
			handler: func() error { return nil },
		},
		"without reply": {
			handler: func(args *struct{}) error { return nil },
		},
		"with a non-pointer reply": {
			handler: func(args, reply struct{}) error { return nil },
		},
		"with non-struct args": {
			handler: func(args int, reply *struct{}) error { return nil },
		},
		"without error": {
			handler: func(args, reply *struct{}) {},
		},
		"with multiple return values": {
			handler: func(args, reply *struct{}) (int, error) { return 0, nil },
		},
		"with multiple return values and non-error": {
			handler: func(args, reply *struct{}) (int, int) { return 0, 0 },
		},
		"with non error return": {
			handler: func(args, reply *struct{}) int { return 0 },
		},
	}

	for name, data := range testData {
		t.Run(name, func(t *testing.T) {
			r := jrpc.NewRegistry()

			err := r.Register("method", data.handler)
			if err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestRegistry_CallShouldReturnErrorWithInvalidMethod(t *testing.T) {
	t.Parallel()

	r := jrpc.NewRegistry()

	_, err := r.Call("method", nil)
	if err == nil {
		t.Error("expected an error, got nil")
	}
}

func TestRegistry_CallShouldCallHandler(t *testing.T) {
	t.Parallel()

	type args struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	type reply struct {
		Greeting string `json:"greeting"`
	}

	r := jrpc.NewRegistry()

	err := r.Register("method", func(args *args, reply *reply) error {
		reply.Greeting = "Hello, " + args.Name

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := r.Call("method", &args{Name: "John", Age: 30})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := result.(*reply)

	want := reply{Greeting: "Hello, John"}

	if got.Greeting != want.Greeting {
		t.Errorf("expected %v, got %v", want.Greeting, got.Greeting)
	}
}

func TestRegistry_RegisterShouldBeGoroutineSafe(t *testing.T) {
	t.Parallel()

	r := jrpc.NewRegistry()

	const n = 1000

	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			_ = r.Register("method", func(args, reply *struct{}) error { return nil })

			wg.Done()
		}()
	}

	wg.Wait()
}

func TestRegistry_CallShouldBeGoroutineSafeWithANonExistingMethod(t *testing.T) {
	t.Parallel()

	r := jrpc.NewRegistry()

	const n = 1000

	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			_, _ = r.Call("method", nil)

			wg.Done()
		}()
	}

	wg.Wait()
}

func TestRegistry_CallShouldBeGoroutineSafeWithAnExistingMethod(t *testing.T) {
	t.Parallel()

	r := jrpc.NewRegistry()

	err := r.Register("method", func(args, reply *struct{}) error { return nil })
	if err != nil {
		t.Fatal(err)
	}

	const n = 1000

	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			_, _ = r.Call("method", nil)

			wg.Done()
		}()
	}

	wg.Wait()
}
