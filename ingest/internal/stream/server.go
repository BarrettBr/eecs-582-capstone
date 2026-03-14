package stream

/*
Name: ingest/internal/stream/server.go
Description: Gorilla websocket server for batched ingest and ML messages to the frontend.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-03-14
Revision History:
- 2026-02-28, Barrett Brown: Created websocket stream package with batching support.
- 2026-02-28, Barrett Brown: Swapped manual websocket handling to gorilla websocket.
- 2026-03-14, Barrett Brown: Added service-room subscriptions, registrar-driven pruning, and catalog broadcasts.
- 2026-03-14, Barrett Brown: Removed obsolete unscoped publish wrappers after fully moving to service-room delivery.
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
	"slices"
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
	response  chan subscriptionAckPayload
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

// PublishScopedEvent queues one ingest event for a specific service room.
func (s *Server) PublishScopedEvent(eventType, serviceName string, payload []byte, timestamp string) {
	s.Publish(Message{
		Kind:        "event",
		EventType:   eventType,
		Source:      "ingest",
		ServiceName: serviceName,
		Timestamp:   timestamp,
		Data:        payload,
	})
}

// PublishScopedMLResult queues one ML response payload for a specific service room.
func (s *Server) PublishScopedMLResult(eventType, serviceName string, payload []byte) {
	s.Publish(Message{
		Kind:        "ml_result",
		EventType:   eventType,
		Source:      "ml",
		ServiceName: serviceName,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Data:        payload,
	})
}

// ApplyServiceCatalog updates the valid service set, prunes removed subscriptions, and broadcasts the new catalog.
func (s *Server) ApplyServiceCatalog(catalog ServiceCatalogPayload, removed []string) {
	s.controlCh <- applyCatalogRequest{
		catalog: catalog,
		removed: append([]string(nil), removed...),
	}
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
		log.Printf("Stream dropped message kind=%s source=%s service=%s", msg.Kind, msg.Source, msg.ServiceName)
	}
}

func (s *Server) run(ctx context.Context) {
	globalBatch := make([]Message, 0, s.batchSize)
	roomBatches := make(map[string][]Message)
	ticker := time.NewTicker(s.batchFlushInterval)
	defer ticker.Stop()

	flushGlobal := func() {
		if len(globalBatch) == 0 {
			return
		}
		frame, err := marshalBatchPayload(globalBatch)
		if err != nil {
			log.Printf("Stream batch encode error: %v", err)
			globalBatch = globalBatch[:0]
			return
		}
		s.broadcast(frame)
		globalBatch = globalBatch[:0]
	}

	flushRoom := func(serviceName string) {
		batch := roomBatches[serviceName]
		if len(batch) == 0 {
			return
		}
		frame, err := marshalBatchPayload(batch)
		if err != nil {
			log.Printf("Stream room batch encode error service=%s: %v", serviceName, err)
			roomBatches[serviceName] = batch[:0]
			return
		}
		s.broadcastRoom(serviceName, frame)
		roomBatches[serviceName] = batch[:0]
	}

	flushAll := func() {
		flushGlobal()
		for serviceName := range roomBatches {
			flushRoom(serviceName)
		}
	}

	for {
		select {
		case <-ctx.Done():
			flushAll()
			s.closeAllClients()
			return
		case c := <-s.registerCh:
			s.clients[c] = struct{}{}
			s.sendCatalogToClient(c)
		case c := <-s.unregisterCh:
			s.removeClient(c)
		case control := <-s.controlCh:
			switch req := control.(type) {
			case setSubscriptionsRequest:
				s.handleSetSubscriptions(req)
			case applyCatalogRequest:
				s.handleApplyCatalog(req)
			}
		case msg := <-s.publishCh:
			if msg.ServiceName != "" {
				roomBatches[msg.ServiceName] = append(roomBatches[msg.ServiceName], msg)
				if len(roomBatches[msg.ServiceName]) >= s.batchSize {
					flushRoom(msg.ServiceName)
				}
				continue
			}
			globalBatch = append(globalBatch, msg)
			if len(globalBatch) >= s.batchSize {
				flushGlobal()
			}
		case <-ticker.C:
			flushAll()
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

func (s *Server) broadcastRoom(serviceName string, frame []byte) {
	for c := range s.roomMembers[serviceName] {
		s.enqueueFrame(c, frame)
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
	for serviceName := range c.subscriptions {
		s.removeClientFromRoom(c, serviceName)
	}
	delete(s.clients, c)
	close(c.send)
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (s *Server) removeClientFromRoom(c *client, serviceName string) {
	members := s.roomMembers[serviceName]
	if len(members) == 0 {
		delete(c.subscriptions, serviceName)
		return
	}
	delete(members, c)
	if len(members) == 0 {
		delete(s.roomMembers, serviceName)
	}
	delete(c.subscriptions, serviceName)
}

func (s *Server) enqueueFrame(c *client, frame []byte) {
	select {
	case c.send <- frame:
	default:
		log.Printf("Stream dropped slow client session=%s", c.session.ID)
		s.removeClient(c)
	}
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
			log.Printf("Stream read decode error: %v", err)
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

func (s *Server) dispatchIncoming(c *client, msg Message) {
	if msg.Kind == "set_service_subscriptions" {
		var req subscriptionSetRequest
		if len(msg.Data) > 0 {
			if err := json.Unmarshal(msg.Data, &req); err != nil {
				log.Printf("Stream subscription decode error session=%s: %v", c.session.ID, err)
				return
			}
		}
		s.setClientSubscriptions(c, req.ServiceNames)
		return
	}

	s.readHandlerMu.RLock()
	handler, ok := s.readHandlers[msg.Kind]
	s.readHandlerMu.RUnlock()
	if !ok {
		log.Printf("Stream ignored inbound message kind=%s source=%s", msg.Kind, msg.Source)
		return
	}
	handler(s.runCtx, c.session, msg)
}

// URL returns the server websocket path for logging or config display.
func (s *Server) URL() string {
	return fmt.Sprintf("ws://%s%s", s.addr, s.path)
}

func (s *Server) setClientSubscriptions(c *client, requested []string) subscriptionAckPayload {
	response := make(chan subscriptionAckPayload, 1)
	s.controlCh <- setSubscriptionsRequest{
		client:    c,
		requested: append([]string(nil), requested...),
		response:  response,
	}
	return <-response
}

func (s *Server) handleSetSubscriptions(req setSubscriptionsRequest) {
	if _, ok := s.clients[req.client]; !ok {
		if req.response != nil {
			req.response <- subscriptionAckPayload{}
		}
		return
	}

	acceptedSet := make(map[string]struct{})
	rejectedSet := make(map[string]struct{})
	for _, serviceName := range req.requested {
		if _, ok := s.validServices[serviceName]; ok {
			acceptedSet[serviceName] = struct{}{}
			continue
		}
		rejectedSet[serviceName] = struct{}{}
	}

	for serviceName := range req.client.subscriptions {
		if _, ok := acceptedSet[serviceName]; ok {
			continue
		}
		s.removeClientFromRoom(req.client, serviceName)
	}
	for serviceName := range acceptedSet {
		if _, ok := req.client.subscriptions[serviceName]; ok {
			continue
		}
		if s.roomMembers[serviceName] == nil {
			s.roomMembers[serviceName] = make(map[*client]struct{})
		}
		s.roomMembers[serviceName][req.client] = struct{}{}
		req.client.subscriptions[serviceName] = struct{}{}
	}

	payload := subscriptionAckPayload{
		AcceptedServices: sortedKeys(acceptedSet),
		RejectedServices: sortedKeys(rejectedSet),
		CurrentServices:  sortedKeys(req.client.subscriptions),
	}
	log.Printf("Stream updated subscriptions session=%s current=%d", req.client.session.ID, len(payload.CurrentServices))
	s.sendDirectMessage(req.client, Message{
		Kind:      "subscription_ack",
		Source:    "stream",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Data:      mustMarshalJSON(payload),
	})
	if req.response != nil {
		req.response <- payload
	}
}

func (s *Server) handleApplyCatalog(req applyCatalogRequest) {
	newValidServices := make(map[string]struct{}, len(req.catalog.Services))
	for _, service := range req.catalog.Services {
		newValidServices[service.Name] = struct{}{}
	}

	removedSet := make(map[string]struct{}, len(req.removed))
	for _, serviceName := range req.removed {
		removedSet[serviceName] = struct{}{}
	}
	for serviceName := range s.validServices {
		if _, ok := newValidServices[serviceName]; !ok {
			removedSet[serviceName] = struct{}{}
		}
	}

	type prunedState struct {
		client  *client
		removed map[string]struct{}
	}
	affected := make(map[*client]*prunedState)
	for serviceName := range removedSet {
		for c := range s.roomMembers[serviceName] {
			state := affected[c]
			if state == nil {
				state = &prunedState{
					client:  c,
					removed: make(map[string]struct{}),
				}
				affected[c] = state
			}
			state.removed[serviceName] = struct{}{}
			s.removeClientFromRoom(c, serviceName)
		}
		delete(s.roomMembers, serviceName)
	}

	s.validServices = newValidServices
	s.catalog = ServiceCatalogPayload{
		Revision: req.catalog.Revision,
		Services: append([]ServiceCatalogEntry(nil), req.catalog.Services...),
	}

	for _, state := range affected {
		s.sendDirectMessage(state.client, Message{
			Kind:      "subscriptions_pruned",
			Source:    "stream",
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Data: mustMarshalJSON(subscriptionsPrunedPayload{
				RemovedServices: sortedKeys(state.removed),
				CurrentServices: sortedKeys(state.client.subscriptions),
				Reason:          "service_removed",
			}),
		})
	}

	s.broadcastDirectMessage(Message{
		Kind:      "service_catalog",
		Source:    "stream",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Data:      mustMarshalJSON(s.catalog),
	})
}

func (s *Server) sendCatalogToClient(c *client) {
	if len(s.catalog.Services) == 0 && s.catalog.Revision == "" {
		return
	}
	s.sendDirectMessage(c, Message{
		Kind:      "service_catalog",
		Source:    "stream",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Data:      mustMarshalJSON(s.catalog),
	})
}

func (s *Server) sendDirectMessage(c *client, msg Message) {
	frame, err := marshalBatchPayload([]Message{msg})
	if err != nil {
		log.Printf("Stream direct encode error session=%s kind=%s: %v", c.session.ID, msg.Kind, err)
		return
	}
	s.enqueueFrame(c, frame)
}

func (s *Server) broadcastDirectMessage(msg Message) {
	frame, err := marshalBatchPayload([]Message{msg})
	if err != nil {
		log.Printf("Stream direct broadcast encode error kind=%s: %v", msg.Kind, err)
		return
	}
	s.broadcast(frame)
}

func mustMarshalJSON(payload any) json.RawMessage {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Stream JSON encode error: %v", err)
		return json.RawMessage(`{}`)
	}
	return data
}

func sortedKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return keys
}
