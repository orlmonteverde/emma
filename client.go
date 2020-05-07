package emma

import (
	"net/http"

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
}

// Send message to client.
func (c *Client) Send(msg []byte) {
	c.send <- msg
}

// Broadcast send message to all clients.
func (c *Client) Broadcast(msg []byte) {
	c.Emma.BroadcastFilter(msg, func(client *Client) bool {
		return client != c
	})
}

// Read messages from the socket.
func (c *Client) Read(messageHandler func(c *Client, msg []byte)) {
	defer c.Socket.Close()

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
	defer c.Socket.Close()

	for msg := range c.send {
		err := c.Socket.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return
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
