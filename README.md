# go-jrpc

go-jrpc is a library to make  JSON-RPC 2.0 clients and build servers in Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/kytnacode/go-jrpc.svg)](https://pkg.go.dev/github.com/kytnacode/go-jrpc) 

## Overview

go-jrpc provides a simple server and client compliant with the JSON-RPC 2.0 specification.
go-jrpc provides:
- A server that can handle JSON-RPC 2.0 requests over HTTP, net.Conn and net.Listener.
- A client that can send JSON-RPC 2.0 requests over net.Conn.
- Support for notifications (requests without a response).
- Support for batch requests (multiple requests in a single JSON-RPC call).
- Support for synchronous and asynchronous calls.

## Install

Get the latest version of go-jrpc:

```bash
go get github.com/kytnacode/go-jrpc@latest
```

Next, import the package in your code:

```go
import "github.com/kytnacode/go-jrpc"
```

## Usage

### Server

```go
package main

import (
	"log"
	"net/http"

	"github.com/kytnacode/go-jrpc"
	"github.com/kytnacode/go-jrpc/group"
)

type Args struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Reply struct {
	Result int `json:"result"`
}

func main() {
	// Create a new server.
	s := jrpc.NewServer(log.Printf)

	// Register methods.
	var g group.Group

	g.AddMethod("echo", func(args Args, reply *Reply) error {
		reply.Result = args.A

		return nil
	}) 

	g.AddMethod("add", func(args Args, reply *Reply) error {
		reply.Result = args.A + args.B

		return nil
	})
	g.AddMethod("sub", func(args Args, reply *Reply) error {
		reply.Result = args.A - args.B

		return nil
	})

	for err := range g.RegisterTo(s) {
		log.Printf("error registering method: %v", err)
	}


	// Start the server.

	// Serve via HTTP.
	http.Handle("/rpc", s)
	log.Fatal(http.ListenAndServe(":8080", nil))

	// Serve via net.Listener.
	// l, err := net.Listen("tcp", ":8080")
	// if err != nil {
	// 	log.Fatalf("listening: %v", err)
	// }
	//
	// err := s.Accept(context.Background(), l)
	// if err != nil {
	// 	log.Fatalf("accepting: %v", err)
	// }
}
```

### Client

```go
package main

import (
	"context"
	"log"
	"net"

	"github.com/kytnacode/go-jrpc"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	errCh := make(chan error, 1)

	c := jrpc.NewClient(conn)
	go c.Input(context.Background(), errCh)

	go func() {
		for err := range errCh {
			log.Printf("error: %v", err)
		}
	}()

	call := c.Call(jrpc.Call("echo").Args(struct{ A string }{"hello"}))

	if err := call.Error; err != nil {
		log.Printf("error: %v", err)
		return
	}

	if err := call.Result[0].Error; err != nil {
		log.Printf("error: %v", err)
		return
	}

	// Handle the result
	result := call.Result[0].Result
}
```

for more examples, see [examples](examples) directory.


## License

go-jrpc is released under the MIT License (see [LICENSE](LICENSE)).
