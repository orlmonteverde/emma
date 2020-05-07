package main

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/orlmonteverde/emma"
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

		client := emma.NewClient(ws, r, messageBufferSize)
		e.Add(client)
		defer e.Remove(client)

		go client.Write()
		client.Read(func(c *emma.Client, msg []byte) {
			c.Send(msg)
		})
	})

	mux.Handle("/", http.FileServer(http.Dir("public")))

	go e.Run()

	http.ListenAndServe(":8000", mux)
}
