package emma

import "encoding/json"

// Emma allows you to manage clients connected to the WebSocket.
type Emma struct {
	// join is a channel for clients wishing to join the room.
	join chan *Client
	// leave is a channel for clients wishing to leave the room.
	leave chan *Client
	// clients holds all current clients in this room.
	clients map[*Client]bool
}

// Broadcast sent message to all clients.
func (e *Emma) Broadcast(msg []byte) error {
	for client := range e.clients {
		client.Send(msg)
	}
	return nil
}

// BroadcastFilter sent message to clients with filter.
func (e *Emma) BroadcastFilter(msg []byte, f func(client *Client) bool) error {
	for client := range e.clients {
		if f(client) {
			client.Send(msg)
		}
	}

	return nil

}

// BroadcastJSON sent JSON message to all clients.
func (e *Emma) BroadcastJSON(i interface{}) error {
	msg, err := json.Marshal(i)
	if err != nil {
		return err
	}

	return e.Broadcast(msg)
}

// BroadcastFilterJSON sent JSON message to clients with filter.
func (e *Emma) BroadcastFilterJSON(i interface{}, f func(client *Client) bool) error {
	msg, err := json.Marshal(i)
	if err != nil {
		return err
	}

	return e.BroadcastFilter(msg, f)
}

// Run start the room.
func (e *Emma) Run() {
	for {
		select {
		case client := <-e.join:
			// joining.
			e.clients[client] = true
		case client := <-e.leave:
			// leaving.
			delete(e.clients, client)
			close(client.send)
		}
	}
}

// Add a client.
func (e *Emma) Add(c *Client) {
	c.Emma = e
	e.join <- c
}

// Remove a client.
func (e *Emma) Remove(c *Client) {
	e.leave <- c
}

// New makes a new emma instance.
func New() *Emma {
	return &Emma{
		join:    make(chan *Client),
		leave:   make(chan *Client),
		clients: make(map[*Client]bool),
	}
}
