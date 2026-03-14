package stream

/*
Name: ingest/internal/stream/server_lifecycle.go
Description: Stream server lifecycle, registration, and websocket connection loops.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-03-14
Revision History:
- 2026-02-28, Barrett Brown: Created websocket stream package with batching support.
- 2026-02-28, Barrett Brown: Swapped manual websocket handling to gorilla websocket.
- 2026-03-14, Barrett Brown: Added service-room subscriptions, registrar-driven pruning, and catalog broadcasts.
- 2026-03-14, Barrett Brown: Split the larger stream server into focused modules for lifecycle, batching, and subscriptions.
Preconditions:
- The server is constructed before lifecycle methods or handler registration are used.
Postconditions:
- Start runs the batch loop and websocket listener until the context is canceled.
Known Faults:
- Client disconnects are mainly detected on read or write attempts.
*/

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Start begins the websocket batch loop and HTTP listener.
func (s *Server) Start(ctx context.Context) error {
	s.runCtx = ctx
	go s.run(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
	}()

	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// RegisterReadHandler adds or replaces the handler for one inbound message kind.
func (s *Server) RegisterReadHandler(kind string, handler ReadHandler) {
	if kind == "" || handler == nil {
		return
	}

	s.readHandlerMu.Lock()
	defer s.readHandlerMu.Unlock()
	s.readHandlers[kind] = handler
}

// RegisterHTTPHandler adds or replaces one plain HTTP handler on the shared server mux.
func (s *Server) RegisterHTTPHandler(path string, handler http.HandlerFunc) {
	if path == "" || handler == nil || s.mux == nil {
		return
	}
	s.mux.HandleFunc(path, handler)
}

func (s *Server) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &client{
		session: ClientSession{
			ID: fmt.Sprintf("ws-%d", s.nextClientID.Add(1)),
		},
		conn:          conn,
		send:          make(chan []byte, 16),
		subscriptions: make(map[string]struct{}),
	}
	s.registerCh <- c
	go s.readLoop(c)
	go s.writeLoop(c)
}

func (s *Server) readLoop(c *client) {
	for {
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			s.unregisterCh <- c
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		messages, err := parseIncomingPayload(payload)
		if err != nil {
			continue
		}
		for _, msg := range messages {
			s.dispatchIncoming(c, msg)
		}
	}
}

func (s *Server) writeLoop(c *client) {
	for frame := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, frame); err != nil {
			s.unregisterCh <- c
			return
		}
	}
}

// URL returns the server websocket path for logging or config display.
func (s *Server) URL() string {
	return fmt.Sprintf("ws://%s%s", s.addr, s.path)
}
