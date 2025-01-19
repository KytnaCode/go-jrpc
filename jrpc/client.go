package jrpc

import (
	"encoding/json"
	"fmt"
	"net"
)

type Client struct {
	conn net.Conn
}

// NewClient creates a new JSON-RPC 2.0 client over conn.
func NewClient(conn net.Conn) *Client {
	return &Client{conn: conn}
}

// Dial works like net.Dial but instead of returning a net.Conn returns a new JSON-RPC client.
func Dial(network, address string) (*Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("jrpc.Client: could not dial addr: %w", err)
	}

	return &Client{conn: conn}, nil
}

// NewRequest creates a new JSON-RPC request.
func NewRequest(rpcID *int, method string, params json.RawMessage) *Message {
	return &Message{
		JSONRPC: jsonrpc,
		ID:      rpcID,
		Method:  method,
		Params:  params,
		Error:   nil,
		Result:  nil,
	}
}

// Do sends a request and returns its response.
func (c *Client) Do(req *Message) (*Message, error) {
	if err := json.NewEncoder(c.conn).Encode(req); err != nil {
		return nil, fmt.Errorf("could not write request: %w", err)
	}

	var resp Message

	if err := json.NewDecoder(c.conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("could not decode response: %w", err)
	}

	return &resp, nil
}

// DoBatch sends a batch of requests and return its responses.
func (c *Client) DoBatch(req []*Message) ([]*Message, error) {
	if err := json.NewEncoder(c.conn).Encode(req); err != nil {
		return nil, fmt.Errorf("could not write request: %w", err)
	}

	var resp []*Message

	if err := json.NewDecoder(c.conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("could not decode response: %w", err)
	}

	return resp, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("client close error: %w", err)
	}

	return nil
}
