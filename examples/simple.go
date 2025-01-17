package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	"github.com/kytnacode/go-jrpc/jrpc"
)

type ArgsType struct {
	A, B *float64
}

func Mul(args ArgsType) (float64, error) {
	if args.A == nil || args.B == nil {
		return 0, jrpc.NewError(2, "missing operands", nil) //nolint:mnd
	}

	return *args.A * *args.B, nil
}

func Div(args ArgsType) (float64, error) {
	if args.A == nil || args.B == nil {
		return 0, jrpc.NewError(2, "missing operands", nil) //nolint:mnd
	}

	if *args.B == 0 {
		return 0, jrpc.NewError(1, "could not divide by zero", nil)
	}

	return *args.A / *args.B, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	server := jrpc.CreateServerWithLogging(logger)

	logger.Info("registering methods")

	// You can register methods as a map, where the key is the method name and the value the handler.
	err := server.RegisterMap(map[string]any{
		"mul": Mul, // Value must be the handler function itself.
		"div": Div,
	})
	handleError(err)

	l, err := net.Listen("tcp", "127.0.0.1:3000")
	handleError(err)

	logger.Info("Listening on port 3000")
	handleError(server.ServeListener(context.Background(), l))
}

func handleError(err error) {
	if err != nil {
		panic(err)
	}
}
