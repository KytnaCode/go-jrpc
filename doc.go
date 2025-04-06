// Package jrpc provides a JSON-RPC 2.0 implementation for Go.
//   - Provides a client with support for single and batch requests, notifications, synchronous and asynchronous calls.
//   - Provides a server with support for single and batch requests, notifications, and concurrent use.
//
// To create a new server, use the NewServer function:
//
//	server := jrpc.NewServer(log.Printf) // Log to standard output. If log is nil, no logging is done.
//
//	var g group.Group
//
//	g.AddMethod("echo", func(args struct{ A string }, reply *struct{ C string }) error {
//	  reply.C = args.A
//
//	  return nil
//	})
//
//	g.Use("math", func(g *group.Group) {
//	  g.AddMethod("add", func(args struct{ A, B int }, reply *struct{ C int }) error {
//	    reply.C = args.A + args.B
//
//	    return nil
//	   })
//
//	  g.AddMethod("sub", func(args struct{ A, B int }, reply *struct{ C int }) error {
//	    reply.C = args.A - args.B
//
//	    return nil
//	  })
//	})
//
//	g.RegisterTo(server) // Register all methods to the server
//
//	// Start the server
//	// Via TCP
//	lis, err := net.Listen("tcp", ":8080")
//	if err != nil {
//	  log.Fatalf("failed to listen: %v", err)
//	}
//	defer lis.Close()
//
//	if err := server.Accept(context.Background(), lis); err != nil {
//	  log.Fatalf("failed to accept: %v", err)
//	}
//
//	// Via HTTP
//	// http.Handle("/rpc", server)
//	// if err := http.ListenAndServe(":8080", nil); err != nil {
//	// 	log.Fatalf("failed to listen: %v", err)
//	// }
//
// To create a new client, use the NewClient function:
//
//	conn, err := net.Dial("tcp", "localhost:8080")
//	if err != nil {
//		log.Fatalf("failed to connect: %v", err)
//	}
//
//	errCh := make(chan error, 1)
//
//	c := jrpc.NewClient(conn)
//	go c.Input(context.Background(), errCh)
//
//	go func() {
//		for err := range errCh {
//			if err != nil {
//				log.Printf("error: %v", err)
//				continue
//			}
//
//			log.Println("received error:", err)
//		}
//	}()
//
//	call := c.Call(jrpc.Call("echo").Args(struct{ A string }{"hello"}))
//
//	if err := call.Error; err != nil {
//		log.Printf("error: %v", err)
//		return
//	}
//
//	if err := call.Result[0].Error; err != nil {
//		log.Printf("error: %v", err)
//		return
//	}
//
//	fmt.Println("result:", call.Result[0].Result)
package jrpc
