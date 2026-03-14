package stream

/*
Name: ingest/internal/stream/server_subscriptions.go
Description: Stream server inbound command handling, service subscriptions, and registrar-driven catalog updates.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Barrett Brown: Added service-room subscriptions, registrar-driven pruning, and catalog broadcasts.
- 2026-03-14, Barrett Brown: Split the larger stream server into focused modules for lifecycle, batching, and subscriptions.
Preconditions:
- Subscription updates are funneled through the control loop for consistent room index changes.
Postconditions:
- Client room membership and catalog state stay aligned with the current service set.
Known Faults:
- Unknown inbound message kinds are still ignored after a log entry.
*/

import (
	"encoding/json"
	"log"
	"slices"
	"time"
)

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

// ApplyServiceCatalog updates the valid service set, prunes removed subscriptions, and broadcasts the new catalog.
func (s *Server) ApplyServiceCatalog(catalog ServiceCatalogPayload, removed []string) {
	s.controlCh <- applyCatalogRequest{
		catalog: catalog,
		removed: append([]string(nil), removed...),
	}
}

func (s *Server) setClientSubscriptions(c *client, requested []string) {
	s.controlCh <- setSubscriptionsRequest{
		client:    c,
		requested: append([]string(nil), requested...),
	}
}

func (s *Server) handleSetSubscriptions(req setSubscriptionsRequest) {
	if _, ok := s.clients[req.client]; !ok {
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
