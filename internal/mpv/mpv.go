// Package mpv is a minimal client for mpv's JSON IPC (--input-ipc-server).
package mpv

import (
	"bufio"
	"encoding/json"
	"net"
)

// Event is one event line sent by mpv.
type Event struct {
	Event string          `json:"event"`
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"data"`
}

// Client is a connection to one mpv instance.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
}

// Dial connects to a named pipe (Windows) or unix socket (elsewhere).
func Dial(path string) (*Client, error) {
	conn, err := dial(path)
	if err != nil {
		return nil, err
	}
	return &Client{conn, json.NewEncoder(conn)}, nil
}

// Command sends a command without waiting for its reply.
func (c *Client) Command(args ...any) { c.enc.Encode(map[string]any{"command": args}) }

// Events delivers events until the player goes away, then closes the channel.
func (c *Client) Events() <-chan Event {
	ch := make(chan Event, 64)
	go func() {
		sc := bufio.NewScanner(c.conn)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var e Event
			if json.Unmarshal(sc.Bytes(), &e) == nil && e.Event != "" {
				ch <- e
			}
		}
		close(ch)
	}()
	return ch
}
