package sockjsclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"sync"

	"github.com/igm/sockjs-go/sockjs"
)

type XHR struct {
	Address          string
	TransportAddress string
	ServerID         string
	SessionID        string
	Inbound          chan []byte
	Done             chan bool
	sessionState     sockjs.SessionState
	mu               sync.RWMutex
}

var client = http.Client{Timeout: time.Second * 10}

func NewXHR(address string) (*XHR, error) {
	xhr := &XHR{
		Address:      address,
		ServerID:     GenerateServerID(),
		SessionID:    GenerateSessionID(),
		Inbound:      make(chan []byte),
		Done:         make(chan bool),
		sessionState: sockjs.SessionOpening,
	}
	xhr.TransportAddress = address + "/" + xhr.ServerID + "/" + xhr.SessionID
	if err := xhr.Init(); err != nil {
		return nil, err
	}
	go xhr.StartReading()

	return xhr, nil
}

func (x *XHR) Init() error {
	req, err := http.NewRequest("POST", x.TransportAddress+"/xhr", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if body[0] != 'o' {
		return errors.New("Invalid initial message")
	}
	x.setSessionState(sockjs.SessionActive)

	return nil
}

func (x *XHR) StartReading() {
	client := &http.Client{Timeout: time.Second * 30}
	for {
		select {
		case <-x.Done:
			return
		default:
			req, err := http.NewRequest("POST", x.TransportAddress+"/xhr", nil)
			if err != nil {
				log.Print(err)
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				log.Print(err)
				continue
			}

			data, err := ioutil.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				log.Print(err)
				continue
			}

			switch data[0] {
			case 'h':
				// Heartbeat
				continue
			case 'a':
				// Normal message
				x.Inbound <- data[1:]
			case 'c':
				// Session closed
				x.setSessionState(sockjs.SessionClosed)
				var v []interface{}
				if err := json.Unmarshal(data[1:], &v); err != nil {
					log.Printf("Closing session: %s", err)
					break
				}
				log.Printf("%v: %v", v[0], v[1])
			}
		}
	}
}

func (x *XHR) ReadJSON(v interface{}) error {
	message := <-x.Inbound
	return json.Unmarshal(message, v)
}

func (x *XHR) WriteJSON(v interface{}) error {
	message, err := json.Marshal(v)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", x.TransportAddress+"/xhr_send", bytes.NewReader(message))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return errors.New("Invalid HTTP code - " + resp.Status)
	}

	return nil
}

func (x *XHR) Close() error {
	select {
	case x.Done <- true:
	default:
		return fmt.Errorf("Error closing XHR")
	}
	return nil
}

func (x *XHR) GetSessionState() sockjs.SessionState {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.sessionState
}

func (x *XHR) setSessionState(state sockjs.SessionState) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.sessionState = state
}
