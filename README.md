# go-jrpc

JSON-RPC 2.0 client and server implementation in Go.

## Usage

### Server

```go
package main

import (
	//"context"
	"log"
	"net/http"

	"github.com/kytnacode/go-jrpc"
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
	err := jrpc.RegisterInto(s, "add", func(args *Args, reply *Reply) error {
		reply.Result = args.A + args.B

		return nil
	})
	if err != nil {
		log.Fatalf("registering method: %v", err)
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

for more examples, see [examples](examples) directory.

### Client

**TODO**

## License

MIT License (see [LICENSE](LICENSE)).
