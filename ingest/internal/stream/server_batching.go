package stream

/*
Name: ingest/internal/stream/server_batching.go
Description: Stream server batching, room fanout, direct delivery, and JSON framing helpers.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-03-14
Revision History:
- 2026-02-28, Barrett Brown: Created websocket stream package with batching support.
- 2026-03-14, Barrett Brown: Added service-room subscriptions, registrar-driven pruning, and catalog broadcasts.
- 2026-03-14, Barrett Brown: Split the larger stream server into focused modules for lifecycle, batching, and subscriptions.
Preconditions:
- The server run loop owns client maps and room membership while dispatching frames.
Postconditions:
- Published messages are batched once per scope and delivered to matching clients.
Known Faults:
- Slow clients are still removed on enqueue instead of being drained later.
*/

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

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
