package emma

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a single chatting user.
type Client struct {
	// socket is the web socket for this client.
	Socket *websocket.Conn
	// Request.
	Request *http.Request
	// send is a channel on which messages are sent.
	send chan []byte
	// room is the room this client is chatting in.
	Emma *Emma

	// ReadLimit is the maximum message size allowed from the client.
	// If <= 0, no limit is enforced (default Gorilla WebSocket behavior).
	ReadLimit int64
	// PingPeriod is the interval at which ping messages are sent to the peer.
	// If <= 0, no pings are sent.
	PingPeriod time.Duration
	// WriteWait is the time allowed to write a message to the peer.
	// If <= 0, no write deadline is set.
	WriteWait time.Duration
	// PongWait is the time allowed to read the next pong message from the peer.
	// If <= 0, no read deadline is set.
	PongWait time.Duration

	mu     sync.Mutex
	closed bool
}

// Send message to client.
func (c *Client) Send(msg []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- msg:
	default:
		c.Socket.Close()
	}
}

// close closes the client's send channel.
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
	}
}

// Broadcast transmits a message to all clients except the sender.
func (c *Client) Broadcast(msg []byte) {
	c.Emma.BroadcastFilter(msg, func(client *Client) bool {
		return client != c
	})
}

// BroadcastJSON transmits a message in JSON format to all clients except the sender.
func (c *Client) BroadcastJSON(i interface{}) {
	c.Emma.BroadcastFilterJSON(i, func(client *Client) bool {
		return client != c
	})
}

// Read messages from the socket.
func (c *Client) Read(messageHandler func(c *Client, msg []byte)) {
	defer c.Socket.Close()

	if c.ReadLimit > 0 {
		c.Socket.SetReadLimit(c.ReadLimit)
	}

	if c.PongWait > 0 {
		c.Socket.SetReadDeadline(time.Now().Add(c.PongWait))
		c.Socket.SetPongHandler(func(string) error {
			c.Socket.SetReadDeadline(time.Now().Add(c.PongWait))
			return nil
		})
	}

	for {
		_, msg, err := c.Socket.ReadMessage()
		if err != nil {
			return
		}

		messageHandler(c, msg)
	}
}

// Write message to the socket.
func (c *Client) Write() {
	var ticker *time.Ticker
	if c.PingPeriod > 0 {
		ticker = time.NewTicker(c.PingPeriod)
		defer ticker.Stop()
	}
	defer c.Socket.Close()

	for {
		if ticker != nil {
			select {
			case msg, ok := <-c.send:
				if !ok {
					c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				if c.WriteWait > 0 {
					c.Socket.SetWriteDeadline(time.Now().Add(c.WriteWait))
				}
				err := c.Socket.WriteMessage(websocket.TextMessage, msg)
				if err != nil {
					return
				}
			case <-ticker.C:
				if c.WriteWait > 0 {
					c.Socket.SetWriteDeadline(time.Now().Add(c.WriteWait))
				}
				if err := c.Socket.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		} else {
			msg, ok := <-c.send
			if !ok {
				return
			}
			if c.WriteWait > 0 {
				c.Socket.SetWriteDeadline(time.Now().Add(c.WriteWait))
			}
			err := c.Socket.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				return
			}
		}
	}
}

// NewClient makes a new client instance.
func NewClient(ws *websocket.Conn, r *http.Request, messageBufferSize int) *Client {
	return &Client{
		Socket:  ws,
		Request: r,
		send:    make(chan []byte, messageBufferSize),
	}
}
