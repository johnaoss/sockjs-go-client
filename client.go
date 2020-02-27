package sockjsclient

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Client represents the SockJS client.
type Client struct {
	Connection Connection

	WebSockets   bool
	Address      string
	ReadBufSize  int
	WriteBufSize int

	Reconnected    chan struct{}
	ConnectionLost chan struct{}
}

func NewClient(address string) (*Client, error) {
	client := &Client{
		Address: address,
	}

	// Get info whether WebSockets are enabled
	info, err := client.Info()
	if err != nil {
		return nil, err
	}

	client.WebSockets = info.WebSocket

	// Create a Raw Websocket session (not a SockJS one)
	if client.WebSockets {
		newAddr := strings.Replace(address, "https", "wss", 1)
		newAddr = strings.Replace(newAddr, "http", "ws", 1)

		ws, err := NewWebSocket(newAddr)
		if err != nil {
			return nil, err
		}

		client.Connection = ws
		client.Reconnected = ws.Reconnected
		client.ConnectionLost = ws.ConnectionLost
	} else {
		// Otherwise we're doing XHR Polling
		client.Connection, err = NewXHR(address)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

// Info returns the necessary information about a Client
func (c *Client) Info() (*Info, error) {
	resp, err := http.Get(c.Address + "/info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	var info *Info
	if err := dec.Decode(&info); err != nil {
		return nil, err
	}

	return info, nil
}

func (c *Client) WriteMessage(p interface{}) error {
	return c.Connection.WriteJSON(p)
}

func (c *Client) ReadMessage(p interface{}) error {
	return c.Connection.ReadJSON(p)
}
