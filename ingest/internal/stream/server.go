package stream

/*
Name: ingest/internal/stream/server.go
Description: Gorilla websocket server core types and constructor for batched ingest and ML messages to the frontend.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-03-14
Revision History:
- 2026-02-28, Barrett Brown: Created websocket stream package with batching support.
- 2026-02-28, Barrett Brown: Swapped manual websocket handling to gorilla websocket.
- 2026-03-14, Barrett Brown: Added service-room subscriptions, registrar-driven pruning, and catalog broadcasts.
- 2026-03-14, Barrett Brown: Removed obsolete unscoped publish wrappers after fully moving to service-room delivery.
- 2026-03-14, Barrett Brown: Split the larger stream server into focused modules for lifecycle, batching, and subscriptions.
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
	"net/http"
	"sync"
	"sync/atomic"
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
	Kind        string          `json:"kind"`
	EventType   string          `json:"event_type,omitempty"`
	Source      string          `json:"source"`
	ServiceName string          `json:"service_name,omitempty"`
	Timestamp   string          `json:"timestamp"`
	Data        json.RawMessage `json:"data"`
}

type batchPayload struct {
	Kind     string    `json:"kind"`
	Messages []Message `json:"messages"`
}

// ReadHandler processes one inbound websocket message from a connected client.
type ReadHandler func(context.Context, ClientSession, Message)

// ClientSession is the public session identity passed to inbound websocket handlers.
type ClientSession struct {
	ID string `json:"id"`
}

// ServiceCatalogEntry is one selectable ingest service exposed to websocket clients.
type ServiceCatalogEntry struct {
	Name      string `json:"name"`
	AliasName string `json:"alias_name,omitempty"`
	Mode      string `json:"mode"`
	EventType string `json:"event_type"`
}

// ServiceCatalogPayload is the service list snapshot broadcast to websocket clients.
type ServiceCatalogPayload struct {
	Revision string                `json:"revision"`
	Services []ServiceCatalogEntry `json:"services"`
}

type subscriptionSetRequest struct {
	ServiceNames []string `json:"service_names"`
}

type subscriptionAckPayload struct {
	AcceptedServices []string `json:"accepted_services"`
	RejectedServices []string `json:"rejected_services"`
	CurrentServices  []string `json:"current_services"`
}

type subscriptionsPrunedPayload struct {
	RemovedServices []string `json:"removed_services"`
	CurrentServices []string `json:"current_services"`
	Reason          string   `json:"reason"`
}

type client struct {
	session       ClientSession
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[string]struct{}
}

type setSubscriptionsRequest struct {
	client    *client
	requested []string
}

type applyCatalogRequest struct {
	catalog ServiceCatalogPayload
	removed []string
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
	controlCh          chan any
	clients            map[*client]struct{}
	roomMembers        map[string]map[*client]struct{}
	validServices      map[string]struct{}
	catalog            ServiceCatalogPayload
	nextClientID       atomic.Uint64
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
		controlCh:          make(chan any, 64),
		clients:            make(map[*client]struct{}),
		roomMembers:        make(map[string]map[*client]struct{}),
		validServices:      make(map[string]struct{}),
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
