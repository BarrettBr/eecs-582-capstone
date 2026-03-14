package runtime

/*
Name: ingest/internal/ingest/runtime/buffer_manager_queue.go
Description: BufferManager queue admission helpers, pressure snapshots, and chunked service queue primitives.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added Base logic.
- 2026-03-13, Barrett Brown: Added clearer BufferManager function comments.
- 2026-03-14, Barrett Brown: Split the larger buffer manager into focused modules for runtime flow and queue mechanics.
Preconditions:
- Callers hold the BufferManager mutex before using the locked helper methods in this file.
Postconditions:
- Queue usage counters and chunk storage stay aligned with the buffered event contents.
Known Faults:
- The manager still treats every event as one unit until weighted event sizing is added.
*/

import "slices"

// description: Tries to place one event into the owning service queue.
// input: service queue state and buffered event metadata.
// output: Returns true when the event is admitted.
func (m *BufferManager) admitLocked(service *serviceQueueState, item bufferedEvent) bool {
	if service == nil {
		return false
	}

	for {
		reserved := service.definition.FlowControl.ReservedUnits
		if service.usedUnits+item.units <= reserved || m.sharedUsedUnits+item.units <= m.sharedUnits {
			m.enqueueLocked(service, item)
			return true
		}
		if service.definition.FlowControl.OverloadPolicy != "drop_oldest" || service.queueLen == 0 {
			service.dropped++
			return false
		}
		if !m.dropOldestLocked(service) {
			service.dropped++
			return false
		}
	}
}

func (m *BufferManager) enqueueLocked(service *serviceQueueState, item bufferedEvent) {
	service.queue.pushBack(item, m.chunkSize)
	service.queueLen++
	m.adjustServiceUsageLocked(service, item.units)
	if !service.active {
		service.active = true
		m.activeServices = append(m.activeServices, service.definition.Name)
	}
}

func (m *BufferManager) dropOldestLocked(service *serviceQueueState) bool {
	item, ok := service.queue.popFront()
	if !ok {
		return false
	}
	service.queueLen--
	service.evicted++
	m.adjustServiceUsageLocked(service, -item.units)
	if service.queueLen == 0 {
		service.active = false
		m.activeServices = slices.DeleteFunc(m.activeServices, func(candidate string) bool {
			return candidate == service.definition.Name
		})
	}
	return true
}

func (m *BufferManager) adjustServiceUsageLocked(service *serviceQueueState, deltaUnits int) {
	beforeBorrowed := borrowedUnits(service)
	service.usedUnits += deltaUnits
	m.usedUnits += deltaUnits
	afterBorrowed := borrowedUnits(service)
	m.sharedUsedUnits += afterBorrowed - beforeBorrowed
}

// description: Decides whether fixed rate sampling is active for one service.
// input: service queue state under the manager lock.
// output: Returns true when the hybrid sampling conditions are met.
func (m *BufferManager) samplingActiveLocked(service *serviceQueueState) bool {
	if service == nil {
		return false
	}
	if !service.definition.FlowControl.AdaptiveEnabledValue() {
		return false
	}
	if service.definition.FlowControl.SupportsBackpressureValue(service.definition.Mode) {
		return false
	}
	if m.pressureState != PressureHigh && m.pressureState != PressureCritical {
		return false
	}
	if m.sharedUnits <= 0 {
		return false
	}
	if float64(m.sharedUsedUnits)/float64(m.sharedUnits) < m.runtime.SamplingSharedThreshold {
		return false
	}
	return borrowedUnits(service) > 0
}

func (m *BufferManager) snapshotLocked() bufferSnapshot {
	services := make(map[string]bufferServiceSnapshot, len(m.services))
	for name, service := range m.services {
		services[name] = bufferServiceSnapshot{
			ReservedUnits: service.definition.FlowControl.ReservedUnits,
			BufferedUnits: service.usedUnits,
			QueueDepth:    service.queueLen,
			Sampling:      m.samplingActiveLocked(service),
			DroppedEvents: service.dropped,
			EvictedEvents: service.evicted,
			SampledEvents: service.sampled,
		}
	}
	return bufferSnapshot{
		PressureState:       m.pressureState,
		TotalBufferedUnits:  m.usedUnits,
		TotalCapacityUnits:  m.totalReservedUnits + m.sharedUnits,
		SharedUnitsUsed:     m.sharedUsedUnits,
		SharedUnitsCapacity: m.sharedUnits,
		Services:            services,
	}
}

func (m *BufferManager) updatePressureLocked() (bool, PressureState, []func(PressureState)) {
	totalCapacity := m.totalReservedUnits + m.sharedUnits
	ratio := 0.0
	if totalCapacity > 0 {
		ratio = float64(m.usedUnits) / float64(totalCapacity)
	}
	next := nextPressureState(m.pressureState, ratio, m.runtime.Pressure)
	if next == m.pressureState {
		return false, m.pressureState, nil
	}
	m.pressureState = next
	callbacks := append([]func(PressureState){}, m.pressureCallbacks...)
	return true, next, callbacks
}

func (m *BufferManager) signalReady() {
	select {
	case m.readyCh <- struct{}{}:
	default:
	}
}

func (q *serviceQueue) pushBack(item bufferedEvent, chunkSize int) {
	if q.tail == nil || q.tail.tail == len(q.tail.items) {
		chunk := &queueChunk{items: make([]bufferedEvent, chunkSize)}
		if q.tail == nil {
			q.head = chunk
			q.tail = chunk
		} else {
			q.tail.next = chunk
			q.tail = chunk
		}
	}
	q.tail.items[q.tail.tail] = item
	q.tail.tail++
}

func (q *serviceQueue) popFront() (bufferedEvent, bool) {
	if q.head == nil {
		return bufferedEvent{}, false
	}

	item := q.head.items[q.head.head]
	q.head.items[q.head.head] = bufferedEvent{}
	q.head.head++
	if q.head.head == q.head.tail {
		q.head = q.head.next
		if q.head == nil {
			q.tail = nil
		}
	}
	return item, true
}

func borrowedUnits(service *serviceQueueState) int {
	if service == nil {
		return 0
	}
	return maxInt(0, service.usedUnits-service.definition.FlowControl.ReservedUnits)
}

func eventUnits(_ IngressEvent) int {
	return 1
}

func maxInt(lhs, rhs int) int {
	if lhs > rhs {
		return lhs
	}
	return rhs
}
