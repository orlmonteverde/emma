package emma

import (
	"encoding/json"
	"sync"
)

// Emma allows you to manage clients connected to the WebSocket.
type Emma struct {
	// join is a channel for clients wishing to join the room.
	join chan *Client
	// leave is a channel for clients wishing to leave the room.
	leave chan *Client
	// clients holds all current clients in this room.
	clients map[*Client]bool
	// mu protects clients map.
	mu sync.RWMutex
	// quit is a channel to signal shutting down the room.
	quit chan struct{}
	// once ensures quit is closed only once.
	once sync.Once
}

// Broadcast sent message to all clients.
func (e *Emma) Broadcast(msg []byte) error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for client := range e.clients {
		client.Send(msg)
	}
	return nil
}

// BroadcastFilter sent message to clients with filter.
func (e *Emma) BroadcastFilter(msg []byte, f func(client *Client) bool) error {
	e.mu.RLock()
	defer e.mu.RUnlock()
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
	quitChan := e.getQuitChan()
	for {
		select {
		case client := <-e.join:
			// joining.
			e.mu.Lock()
			e.clients[client] = true
			e.mu.Unlock()
		case client := <-e.leave:
			// leaving.
			e.mu.Lock()
			delete(e.clients, client)
			e.mu.Unlock()
			client.close()
		case <-quitChan:
			e.mu.Lock()
			for client := range e.clients {
				delete(e.clients, client)
				client.close()
			}
			e.mu.Unlock()
			return
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
		quit:    make(chan struct{}),
	}
}

// getQuitChan lazily initializes and returns the quit channel.
func (e *Emma) getQuitChan() chan struct{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.quit == nil {
		e.quit = make(chan struct{})
	}
	return e.quit
}

// Close stops the room and disconnects all clients.
func (e *Emma) Close() {
	e.once.Do(func() {
		close(e.getQuitChan())
	})
}
