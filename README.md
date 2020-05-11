# Emma

**Emma** is a wrapper for the well-known [gorilla/websocket](https://github.com/gorilla/websocket) package, and inspired by the [olahol/melody](https://github.com/olahol/melody) package. **Emma** makes working with WebSockets easy, eliminating repetitive and tedious parts so you can focus on the fun stuff.

![Go](https://img.shields.io/badge/Golang-1.14-blue.svg?logo=go&longCache=true&style=flat)

This package makes use of the Go standard library and the [gorilla/websocket](https://github.com/gorilla/websocket) package.

Internally, the [context](https://golang.org/pkg/context/) package and [net/http](https://golang.org/pkg/net/http/) package are used, in addition to the **channels** to handle the client session. The routines are not used internally, they are under the client's control. To see this working, I invite you to head over to the examples section.

## Getting Started

[Go](https://golang.org/) is required in version 1.7 or higher.

### Install

`go get -u github.com/orlmonteverde/emma`

### Features

* [x] **Lightweight**, less than 200 lines of code.
* [x] **Easy** to use.
* [x] **100% compatible** with [net/http](https://golang.org/pkg/net/http/) and [context](https://golang.org/pkg/context/) packages from the **standard library**.
* [x] [gorilla/websocket](https://github.com/gorilla/websocket) is the **only external dependency**.

## Examples

See [_examples/](https://github.com/orlmonteverde/emma/blob/master/_examples/) for a variety of examples.

### Basic use

```go
package main

import (
        "net/http"
        "log"

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
        log.Faltal(http.ListenAndServe(":8000", mux))
}
```

### Example: [Chat](https://github.com/orlmonteverde/emma/blob/master/_examples/chat/)

![Chat](https://raw.githubusercontent.com/orlmonteverde/emma/master/_examples/chat/example.gif)

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/orlmonteverde/emma/tags).

## Authors

**Orlando Monteverde** - *Initial work* - [orlmonteverde](https://github.com/orlmonteverde)
