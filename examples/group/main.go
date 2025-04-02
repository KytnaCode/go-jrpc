package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/kytnacode/go-jrpc"
	"github.com/kytnacode/go-jrpc/group"
)

type Args struct {
	A, B int
}

type Reply struct {
	C int
}

func echo(args Args, reply *Reply) error {
	reply.C = args.A
	return nil
}

func add(args Args, reply *Reply) error {
	reply.C = args.A + args.B
	return nil
}

func sub(args Args, reply *Reply) error {
	reply.C = args.A - args.B
	return nil
}

func mul(args Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func div(args Args, reply *Reply) error {
	if args.B == 0 {
		return errors.New("division by zero")
	}
	reply.C = args.A / args.B
	return nil
}

func main() {
	s := jrpc.NewServer(log.Printf)

	var g group.Group

	g.AddMethod("echo", echo)

	g.Use("math", func(g *group.Group) {
		g.AddMethod("add", add) // Equivalent to g.Register("math.add", add)
		g.AddMethod("sub", sub) // Equivalent to g.Register("math.sub", sub)
		g.AddMethod("mul", mul) // Equivalent to g.Register("math.mul", mul)
		g.AddMethod("div", div) // Equivalent to g.Register("math.div", div)
	})

	errs := g.RegisterTo(s) // Register all methods of the group to the server
	if len(errs) > 0 {      // Handle all errors
		for _, err := range errs {
			log.Println(err)
		}
		return
	}

	mux := http.NewServeMux()
	mux.Handle("/rpc", s)

	log.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
