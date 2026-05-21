# Emma WebSocket Library Developer & Agent Guide

This document provides a comprehensive overview of the `emma` WebSocket client-management package, detailing its architecture, concurrency model, API contracts, lifecycle, and implementation guidelines.

## Package Overview

`emma` is a lightweight, thread-safe WebSocket client manager written in Go. It manages client registries, handles reading/writing loops, and facilitates filtered or global broadcasting.

---

## Core Components

The package consists of two primary structs:

### 1. `Emma` (Hub/Room Manager)
Manages the registry of active clients and handles joining/leaving events in a serialized background loop.
* **Fields**:
  * `join chan *Client`: Channel to register new clients.
  * `leave chan *Client`: Channel to unregister clients.
  * `clients map[*Client]bool`: Internal registry of active connections.
  * `mu sync.RWMutex`: Lock protecting `clients` from concurrent reads (broadcasting) and writes (joins/leaves).
* **Key Methods**:
  * `Run()`: Serializes join/leave events. Must be run in a separate goroutine.
  * `Add(*Client)`: Enqueues a client to join.
  * `Remove(*Client)`: Enqueues a client to leave.
  * `Broadcast(msg []byte)` / `BroadcastFilter(msg []byte, f func(*Client) bool)`: Thread-safe message broadcasting.

### 2. `Client` (Connection Representation)
Represents a single active WebSocket connection, enclosing the socket read/write loops.
* **Fields**:
  * `Socket *websocket.Conn`: The underlying Gorilla WebSocket connection.
  * `Request *http.Request`: The HTTP upgrade request (useful for context/auth metadata).
  * `send chan []byte`: Buffered channel for outbound messages.
  * `mu sync.Mutex`: Lock protecting the client's write state and channel lifecycle.
  * `closed bool`: Flag indicating if the client's `send` channel is closed.
* **Key Methods**:
  * `Send(msg []byte)`: Safely enqueues a message for transmission without blocking the caller. If the buffer is full, it closes the socket.
  * `Read(handler func(*Client, []byte))`: Reads messages from the socket synchronously. Typically blocks the calling goroutine.
  * `Write()`: Reads outbound messages from the internal `send` channel and writes them to the WebSocket connection. Runs in a separate goroutine.

---

## Concurrency & Thread-Safety Model

The library is designed for safe execution in high-concurrency environments (e.g., handling hundreds of requests/Goroutines concurrently).

### 1. Map Protection (`Emma.mu`)
* To prevent `fatal error: concurrent map iteration and map write` crashes, `Emma.clients` is guarded by `sync.RWMutex`.
* **Writes**: `Run()` acquires a write-lock (`mu.Lock()`) when inserting or deleting clients from the map.
* **Reads**: `Broadcast` and `BroadcastFilter` acquire a read-lock (`mu.RLock()`) when iterating over the map.

### 2. Channel Lifecycle Protection (`Client.mu`)
* Writing to or closing a closed channel causes a panic in Go.
* `Client.mu` synchronizes calls to `Send()` and the internal `close()` method.
* `Send()` checks `c.closed` under lock. If `false`, it attempts to write to `c.send` in a non-blocking `select`. If `true`, it ignores the write.
* `close()` checks `c.closed` under lock, setting it to `true` and closing `c.send` exactly once.

### 3. Non-Blocking Sends & Slow-Client Eviction
* In `Client.Send()`, if `c.send` is full, the `select` statement falls through to the `default` branch, which calls `c.Socket.Close()`.
* This prevents slow or dead connections from blocking broadcasters (like a Redis message loop). Closing the socket triggers an error in the client's `Read` / `Write` loops, leading to clean teardown and removal from the room.

---

## Connection Lifecycle

1. **Instantiation**: `Client` is created via `NewClient(ws, req, bufferSize)`.
2. **Registration**: The user calls `Emma.Add(client)`. This adds the client to the `join` queue.
3. **Execution Loops**:
   * `go client.Write()` runs in the background, listening to `client.send`.
   * `client.Read(...)` is called synchronously in the handler, blocking until the client disconnects.
4. **Teardown**:
   * Upon disconnect/socket closure, `client.Read(...)` returns.
   * `Emma.Remove(client)` is triggered (typically via `defer`).
   * `Emma.Run()` deletes the client from the map under `mu.Lock()` and invokes `client.close()`.
   * `client.close()` safely closes the channel under `c.mu.Lock()`, prompting `client.Write()` to terminate cleanly.

---

## Guidelines for Agents & Developers

When modifying or extending this codebase, adhere to the following rules:

1. **Do Not Block Under Locks**: Never perform network I/O or block on channel operations while holding `Emma.mu` or `Client.mu`. Keep locked blocks small and deterministic.
2. **Maintain API Contracts**: Never change existing public signatures of `Emma` or `Client` methods, as other production systems depend on them.
3. **Use Safe Channel Clobbering**: Always use the internal `client.close()` method instead of directly closing `client.send` to prevent double-close panics.
4. **Thread-Safe WebSocket Calls**: Gorilla WebSocket connections only support one concurrent writer and reader. `Close()` is safe to call concurrently, which is why it is used to interrupt blocked reads/writes.
