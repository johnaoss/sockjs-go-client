package sockjsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dchest/uniuri"
	"github.com/gorilla/websocket"
)

var (
	ErrClosedConnection = errors.New("use of closed network connection")
)

type ErrNoHeartbeat struct {
	timeout time.Duration
}

func (err ErrNoHeartbeat) Error() string {
	return fmt.Sprintf("No heartbeat was sent for the past %v", err.timeout)
}

type WebSocket struct {
	sync.Mutex
	Address          string
	TransportAddress string
	ServerID         string
	SessionID        string
	Connection       *websocket.Conn
	Inbound          chan []byte
	Reconnected      chan struct{}
	ConnectionLost   chan struct{}
}

func NewWebSocket(address string) (*WebSocket, error) {
	ws := &WebSocket{
		Address:        address,
		ServerID:       paddedRandomIntn(999),
		SessionID:      uniuri.New(),
		Inbound:        make(chan []byte),
		Reconnected:    make(chan struct{}, 32),
		ConnectionLost: make(chan struct{}, 1),
	}

	ws.TransportAddress = address + "/" + ws.ServerID + "/" + ws.SessionID + "/websocket"

	ws.Loop()

	return ws, nil
}

func (w *WebSocket) Loop() {
	go w.loop()

	<-w.Reconnected
}

func (w *WebSocket) loop() {
	defer close(w.Inbound)
	reconnectInterval := time.Second * 5
	for {
		err := w.run()
		if err == nil {
			return
		}
		log.Printf("Websocket disconnected: '%s', attempting to reconnect in %v", err, reconnectInterval)
		time.Sleep(reconnectInterval)
	}
}

type readMessageResponse struct {
	data []byte
	err  error
}

func (w *WebSocket) run() (err error) {
	connectionEstablished := false
	defer func() {
		if connectionEstablished {
			select {
			case w.ConnectionLost <- struct{}{}:
				connectionEstablished = false
			default:
			}
		}
		if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
			err = nil
		}
	}()
	log.Printf("Starting a WebSocket connection to %s", w.TransportAddress)

	ws, _, err := websocket.DefaultDialer.Dial(w.TransportAddress, http.Header{})
	if err != nil {
		return err
	}
	defer ws.Close()

	// Read the open message
	data, err := readMessage(ws)
	if err != nil {
		return err
	}

	if data[0] != 'o' {
		return errors.New("Invalid initial message")
	}

	w.Connection = ws

	w.Reconnected <- struct{}{}
	connectionEstablished = true

	for {
		data, err := readMessage(ws)
		if err != nil {
			return err
		}

		if len(data) < 1 {
			continue
		}

		switch data[0] {
		case 'h':
			// Heartbeat
			continue
		case 'a':
			// Normal message
			w.Inbound <- data[1:]
		case 'c':
			// Session closed
			var v []interface{}
			if err := json.Unmarshal(data[1:], &v); err != nil {
				log.Printf("Closing session: %s", err)
				return nil
			}
			break
		}
	}
}

func readMessage(ws *websocket.Conn) ([]byte, error) {
	timeout := time.Second * 30
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	resCh := make(chan readMessageResponse)
	go func() {
		_, data, err := ws.ReadMessage()
		res := readMessageResponse{
			data: data,
			err:  err,
		}
		select {
		case <-ctx.Done():
		case resCh <- res:
		}
	}()
	select {
	case <-ctx.Done():
		return nil, ErrNoHeartbeat{timeout: timeout}
	case res := <-resCh:
		return res.data, res.err
	}
}

func (w *WebSocket) ReadJSON(v interface{}) error {
	message, ok := <-w.Inbound
	if !ok {
		return ErrClosedConnection
	}
	return json.Unmarshal(message, v)
}

func (w *WebSocket) WriteJSON(v interface{}) error {
	return w.Connection.WriteJSON(v)
}

func (w *WebSocket) Close() error {
	return w.Connection.Close()
}
