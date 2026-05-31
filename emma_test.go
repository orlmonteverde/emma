package emma

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TestEmmaConcurrency(t *testing.T) {
	e := New()
	go e.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		client := NewClient(ws, r, 10)
		e.Add(client)
		defer e.Remove(client)

		go client.Write()
		client.Read(func(c *Client, msg []byte) {
			c.Broadcast(msg)
		})
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	u.Scheme = "ws"

	const numClients = 30
	const numMessages = 100

	var wg sync.WaitGroup
	wg.Add(numClients)

	// Spin up multiple concurrent clients
	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			defer wg.Done()

			conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				t.Errorf("Client %d failed to connect: %v", clientID, err)
				return
			}
			defer conn.Close()

			// Reader loop for client
			go func() {
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						return
					}
				}
			}()

			// Send messages
			for j := 0; j < numMessages; j++ {
				err := conn.WriteMessage(websocket.TextMessage, []byte("hello"))
				if err != nil {
					return
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Also have a concurrent goroutine doing direct Broadcast/BroadcastFilter on e
	stopBroadcast := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopBroadcast:
				return
			default:
				_ = e.Broadcast([]byte("system broadcast"))
				_ = e.BroadcastFilter([]byte("filter broadcast"), func(c *Client) bool {
					return true
				})
				time.Sleep(time.Millisecond * 2)
			}
		}
	}()

	wg.Wait()
	close(stopBroadcast)
}

func TestEmmaClose(t *testing.T) {
	e := New()
	runFinished := make(chan struct{})
	go func() {
		e.Run()
		close(runFinished)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(ws, r, 10)
		e.Add(client)
		go client.Write()
		client.Read(func(c *Client, msg []byte) {})
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	u.Scheme = "ws"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Give a moment for the client to register.
	time.Sleep(10 * time.Millisecond)

	// Close Emma room.
	e.Close()

	// Wait for e.Run to finish.
	select {
	case <-runFinished:
	case <-time.After(200 * time.Millisecond):
		t.Error("e.Run() did not exit after Close()")
	}

	// The client connection should be disconnected.
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("Client socket was not closed when Emma room closed")
	}
}

func TestClientReadLimit(t *testing.T) {
	e := New()
	go e.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(ws, r, 10)
		client.ReadLimit = 5 // set read limit to 5 bytes
		e.Add(client)
		defer e.Remove(client)

		go client.Write()
		client.Read(func(c *Client, msg []byte) {})
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	u.Scheme = "ws"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Send message larger than 5 bytes.
	err = conn.WriteMessage(websocket.TextMessage, []byte("too long message"))
	if err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}

	// Verify that the server disconnected the client.
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("Client was not disconnected after sending message exceeding ReadLimit")
	}
}

func TestClientPingPong(t *testing.T) {
	e := New()
	go e.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(ws, r, 10)
		client.PingPeriod = 15 * time.Millisecond
		client.WriteWait = 10 * time.Millisecond
		e.Add(client)
		defer e.Remove(client)

		go client.Write()
		client.Read(func(c *Client, msg []byte) {})
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	u.Scheme = "ws"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	pingReceived := make(chan struct{})
	conn.SetPingHandler(func(string) error {
		close(pingReceived)
		return nil
	})

	// Run reader in the background to handle the ping.
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-pingReceived:
		// Ping received successfully!
	case <-time.After(200 * time.Millisecond):
		t.Error("Did not receive Ping message from client write loop")
	}
}

type TestMessage struct {
	Sender string `json:"sender"`
	Text   string `json:"text"`
}

func TestBroadcastJSON(t *testing.T) {
	e := New()
	go e.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := NewClient(ws, r, 10)
		e.Add(client)
		defer e.Remove(client)

		go client.Write()
		client.Read(func(c *Client, msg []byte) {})
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	u.Scheme = "ws"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	msg := TestMessage{
		Sender: "system",
		Text:   "hello",
	}

	err = e.BroadcastJSON(msg)
	if err != nil {
		t.Fatalf("Failed to broadcast JSON: %v", err)
	}

	_, rMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	var recMsg TestMessage
	err = json.Unmarshal(rMsg, &recMsg)
	if err != nil {
		t.Fatalf("Failed to unmarshal received message: %v", err)
	}

	if recMsg.Sender != "system" || recMsg.Text != "hello" {
		t.Errorf("Received unexpected message content: %+v", recMsg)
	}
}
