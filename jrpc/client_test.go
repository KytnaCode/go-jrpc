package jrpc_test

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/kytnacode/go-jrpc/jrpc"
)

func newClient(t *testing.T) (*jrpc.Client, net.Conn) {
	t.Helper()

	client, server := net.Pipe()

	c := jrpc.NewClient(client)

	return c, server
}

func newID(t *testing.T, id int) *int {
	t.Helper()

	return &id
}

func TestClient_DoConnShouldSendMessage(t *testing.T) {
	t.Parallel()

	client, conn := newClient(t)

	done := make(chan struct{})

	rpcID := 1

	go func() {
		if _, err := client.Do(jrpc.NewRequest(&rpcID, "sum", []byte("[3, 6]"))); err != nil {
			t.Errorf("client.Do return an error: %v", err)
		}

		done <- struct{}{}
	}()

	var req jrpc.Message

	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		t.Errorf("could not decode request: %v", err)
	}

	if *req.ID != rpcID {
		t.Errorf("request id don't match: expected %v actual %v", rpcID, *req.ID)
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Errorf("could not echo request: %v", err)
	}

	<-done
}

func TestClient_DoShouldWorkInMultipleGoroutines(t *testing.T) {
	t.Parallel()

	msgs := []*jrpc.Message{
		jrpc.NewRequest(newID(t, 1), "sum", []byte("[3, 6]")),
		jrpc.NewRequest(newID(t, 2), "mul", []byte("[4, 8]")),
		jrpc.NewRequest(newID(t, 3), "div", []byte("[4, 2]")),
	}

	client, conn := newClient(t)

	for _, msg := range msgs {
		go func() {
			_, err := client.Do(msg)
			if err != nil {
				t.Errorf("could not send request: %v", err)
			}
		}()
	}

	done := make(chan struct{})

	go func() {
		for range msgs {
			var req jrpc.Message

			if err := json.NewDecoder(conn).Decode(&req); err != nil {
				t.Errorf("could not decode request: %v", err)
			}
		}

		done <- struct{}{}
	}()

	timer := time.NewTimer(time.Second)

	select {
	case <-timer.C:
		t.Errorf("timeout: server don't received the messages")
	case <-done:
	}
}

func TestClient_DoBatchConnShouldSendMessage(t *testing.T) {
	t.Parallel()

	client, conn := newClient(t)

	done := make(chan struct{})

	batch := []*jrpc.Message{
		jrpc.NewRequest(newID(t, 1), "sum", []byte("[2, 4]")),
		jrpc.NewRequest(newID(t, 2), "sub", []byte("[5, 1]")),
		jrpc.NewRequest(newID(t, 3), "mul", []byte("[2, 2]")),
	}

	go func() {
		if _, err := client.DoBatch(batch); err != nil {
			t.Errorf("client.Do return an error: %v", err)
		}

		done <- struct{}{}
	}()

	var req []*jrpc.Message

	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		t.Errorf("could not decode request: %v", err)
	}

	if len(req) != len(batch) {
		t.Errorf("request batch size don't match: expected %v actual %v", len(batch), len(req))
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Errorf("could not echo request: %v", err)
	}

	<-done
}

func TestClient_DoBatchShouldWorkInMultipleGoroutines(t *testing.T) {
	t.Parallel()

	batches := [][]*jrpc.Message{
		{
			jrpc.NewRequest(newID(t, 1), "sum", []byte("[3, 6]")),
			jrpc.NewRequest(newID(t, 2), "mul", []byte("[4, 8]")),
			jrpc.NewRequest(newID(t, 3), "div", []byte("[4, 2]")),
		},
		{
			jrpc.NewRequest(newID(t, 4), "sum", []byte("[3, 6]")),
			jrpc.NewRequest(newID(t, 5), "mul", []byte("[4, 8]")),
			jrpc.NewRequest(newID(t, 6), "div", []byte("[4, 2]")),
		},
		{
			jrpc.NewRequest(newID(t, 7), "sum", []byte("[3, 6]")),
			jrpc.NewRequest(newID(t, 8), "mul", []byte("[4, 8]")),
			jrpc.NewRequest(newID(t, 9), "div", []byte("[4, 2]")),
		},
	}

	client, conn := newClient(t)

	for _, batch := range batches {
		go func() {
			_, err := client.DoBatch(batch)
			if err != nil {
				t.Errorf("could not send request: %v", err)
			}
		}()
	}

	done := make(chan struct{})

	go func() {
		for range batches {
			var req []*jrpc.Message

			if err := json.NewDecoder(conn).Decode(&req); err != nil {
				t.Errorf("could not decode request: %v", err)
			}
		}

		done <- struct{}{}
	}()

	timer := time.NewTimer(time.Second)

	select {
	case <-timer.C:
		t.Errorf("timeout: server don't received the messages")
	case <-done:
	}
}
