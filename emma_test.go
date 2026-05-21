package emma

import (
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
