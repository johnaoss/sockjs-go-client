package sockjsclient

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/cenkalti/backoff"
	"github.com/dchest/uniuri"
	"github.com/gorilla/websocket"
)

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
	go func() {
		connectionLost := false
		err := backoff.Retry(func() error {
			defer func() {
				if !connectionLost {
					select {
					case w.ConnectionLost <- struct{}{}:
						connectionLost = true
					default:
					}
				}
			}()
			log.Printf("Starting a WebSocket connection to %s", w.TransportAddress)

			ws, _, err := websocket.DefaultDialer.Dial(w.TransportAddress, http.Header{})
			if err != nil {
				return err
			}

			// Read the open message
			_, data, err := ws.ReadMessage()
			if err != nil {
				return err
			}

			if data[0] != 'o' {
				return errors.New("Invalid initial message")
			}

			w.Connection = ws

			w.Reconnected <- struct{}{}
			connectionLost = false

			for {
				_, data, err := w.Connection.ReadMessage()
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

			return errors.New("Connection closed")
		}, backoff.NewExponentialBackOff())
		if err != nil {
			log.Print(err)
		}
	}()

	<-w.Reconnected
}

func (w *WebSocket) ReadJSON(v interface{}) error {
	message := <-w.Inbound
	return json.Unmarshal(message, v)
}

func (w *WebSocket) WriteJSON(v interface{}) error {
	return w.Connection.WriteJSON(v)
}

func (w *WebSocket) Close() error {
	return w.Connection.Close()
}
