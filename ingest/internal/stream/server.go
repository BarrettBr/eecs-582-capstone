package stream

/*
Name: ingest/internal/stream/server.go
Description: Gorilla websocket server for batched ingest and ML messages to the frontend.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created websocket stream package with batching support.
- 2026-02-28, Barrett Brown: Swapped manual websocket handling to gorilla websocket.
Preconditions:
- HTTP server address is valid.
- Clients connect using websocket upgrade requests handled by gorilla websocket.
Acceptable Input Values/Types:
- JSON payload bytes for event and ML messages.
- Positive batch sizes and flush intervals.
Unacceptable Input Values/Types:
- Empty websocket path.
- Nil or malformed websocket upgrade requests.
Postconditions:
- Connected clients receive batched websocket messages.
Return Values/Types:
- NewServer: *Server
- Start: error
- Publish methods return no value.
Error/Exception Conditions:
- HTTP listener startup failures.
- Websocket upgrade or write failures.
Side Effects:
- Starts HTTP server, upgrades connections, writes websocket messages.
Invariants:
- Slow clients are dropped instead of blocking the batch loop.
- Publish is non blocking.
Known Faults:
- Client disconnects are mainly detected on write attempts.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Config holds websocket listener and batching settings.
type Config struct {
	Addr               string
	Path               string
	BatchSize          int
	BatchFlushInterval time.Duration
}

// Message is one websocket payload item sent to the frontend.
type Message struct {
	Kind      string          `json:"kind"`
	EventType string          `json:"event_type,omitempty"`
	Source    string          `json:"source"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type batchPayload struct {
	Kind     string    `json:"kind"`
	Messages []Message `json:"messages"`
}

// ReadHandler processes one inbound websocket message from a connected client.
type ReadHandler func(context.Context, Message)

type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Server manages websocket clients and batched outbound messages.
type Server struct {
	addr               string
	path               string
	batchSize          int
	batchFlushInterval time.Duration
	httpServer         *http.Server
	mux                *http.ServeMux
	publishCh          chan Message
	registerCh         chan *client
	unregisterCh       chan *client
	clients            map[*client]struct{}
	upgrader           websocket.Upgrader
	readHandlers       map[string]ReadHandler
	readHandlerMu      sync.RWMutex
	runCtx             context.Context
}

// NewServer builds a websocket server with batching defaults.
func NewServer(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8080"
	}
	if cfg.Path == "" {
		cfg.Path = "/ws"
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 32
	}
	if cfg.BatchFlushInterval <= 0 {
		cfg.BatchFlushInterval = 50 * time.Millisecond
	}

	s := &Server{
		addr:               cfg.Addr,
		path:               cfg.Path,
		batchSize:          cfg.BatchSize,
		batchFlushInterval: cfg.BatchFlushInterval,
		publishCh:          make(chan Message, 256),
		registerCh:         make(chan *client),
		unregisterCh:       make(chan *client),
		clients:            make(map[*client]struct{}),
		readHandlers:       make(map[string]ReadHandler),
		runCtx:             context.Background(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handleWebsocket)
	s.mux = mux
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	return s
}

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

// Publish queues one message for websocket delivery to connected clients.
func (s *Server) Publish(msg Message) {
	msg.Data = append([]byte(nil), msg.Data...)
	s.publish(msg)
}

// PublishEvent queues one ingest event for the frontend.
func (s *Server) PublishEvent(eventType string, payload []byte, timestamp string) {
	s.Publish(Message{
		Kind:      "event",
		EventType: eventType,
		Source:    "ingest",
		Timestamp: timestamp,
		Data:      payload,
	})
}

// PublishMLResult queues one ML response payload for the frontend.
func (s *Server) PublishMLResult(eventType string, payload []byte) {
	s.Publish(Message{
		Kind:      "ml_result",
		EventType: eventType,
		Source:    "ml",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Data:      payload,
	})
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

func (s *Server) publish(msg Message) {
	select {
	case s.publishCh <- msg:
	default:
		log.Printf("Stream dropped message kind=%s source=%s", msg.Kind, msg.Source)
	}
}

func (s *Server) run(ctx context.Context) {
	batch := make([]Message, 0, s.batchSize)
	ticker := time.NewTicker(s.batchFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		frame, err := marshalBatchPayload(batch)
		if err != nil {
			log.Printf("Stream batch encode error: %v", err)
			batch = batch[:0]
			return
		}
		s.broadcast(frame)
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			s.closeAllClients()
			return
		case c := <-s.registerCh:
			s.clients[c] = struct{}{}
		case c := <-s.unregisterCh:
			s.removeClient(c)
		case msg := <-s.publishCh:
			batch = append(batch, msg)
			if len(batch) >= s.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Server) broadcast(frame []byte) {
	for c := range s.clients {
		select {
		case c.send <- frame:
		default:
			s.removeClient(c)
		}
	}
}

func (s *Server) closeAllClients() {
	for c := range s.clients {
		s.removeClient(c)
	}
}

func (s *Server) removeClient(c *client) {
	if _, ok := s.clients[c]; !ok {
		return
	}
	delete(s.clients, c)
	close(c.send)
	_ = c.conn.Close()
}

func (s *Server) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &client{
		conn: conn,
		send: make(chan []byte, 16),
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
			log.Printf("Stream read decode error: %v", err)
			continue
		}
		for _, msg := range messages {
			s.dispatchIncoming(msg)
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

func marshalBatchPayload(messages []Message) ([]byte, error) {
	return json.Marshal(batchPayload{
		Kind:     "batch",
		Messages: messages,
	})
}

func parseIncomingPayload(payload []byte) ([]Message, error) {
	var batch batchPayload
	if err := json.Unmarshal(payload, &batch); err == nil && batch.Kind == "batch" {
		return batch.Messages, nil
	}

	var message Message
	if err := json.Unmarshal(payload, &message); err != nil {
		return nil, err
	}
	return []Message{message}, nil
}

func (s *Server) dispatchIncoming(msg Message) {
	s.readHandlerMu.RLock()
	handler, ok := s.readHandlers[msg.Kind]
	s.readHandlerMu.RUnlock()
	if !ok {
		log.Printf("Stream ignored inbound message kind=%s source=%s", msg.Kind, msg.Source)
		return
	}
	handler(s.runCtx, msg)
}

// URL returns the server websocket path for logging or config display.
func (s *Server) URL() string {
	return fmt.Sprintf("ws://%s%s", s.addr, s.path)
}
