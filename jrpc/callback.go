package jrpc

import "context"

type CallbackHandler interface {
	HandleMsg(ctx context.Context, msg Message) Message
	HandleBatch(ctx context.Context, batch []Message) []Message
}
