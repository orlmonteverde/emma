package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/orlmonteverde/emma"
)

// Message from the websocket connection.
type Message struct {
	Type   MessageType `json:"type,omitempty"`
	Sender string      `json:"sender,omitempty"`
	Text   string      `json:"text,omitempty"`
}

// MessageType is a type of websocket message.
type MessageType uint

// message types
const (
	_ MessageType = iota
	IsTyping
	New
)

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  socketBufferSize,
	WriteBufferSize: messageBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	e := emma.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		nickname := r.URL.Query().Get("nickname")
		if nickname == "" {
			nickname = "Unknown"
		}

		ctx := context.WithValue(r.Context(), "nickname", nickname)
		client := emma.NewClient(ws, r.WithContext(ctx), messageBufferSize)
		e.Add(client)
		defer e.Remove(client)

		go client.Write()
		client.Read(func(c *emma.Client, msg []byte) {
			var m Message
			if err := json.Unmarshal(msg, &m); err != nil {
				return
			}

			iNick := c.Request.Context().Value("nickname")
			nick, ok := iNick.(string)
			if !ok {
				return
			}

			m.Sender = nick

			if m.Type == IsTyping {
				m.Text = fmt.Sprintf("%s is typing...", nick)
			}

			c.BroadcastJSON(m)
		})
	})

	mux.Handle("/", http.FileServer(http.Dir("public")))

	go e.Run()

	log.Fatal(http.ListenAndServe(":8000", mux))
}
