package runtime

/*
Name: ingest/internal/ingest/runtime/buffer_manager.go
Description: Buffers ingress events per service, enforces shared unit limits, and dispatches events fairly into the shared pipeline.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-13
Revision History:
- 2026-03-07, Barrett Brown: Added Base logic.
- 2026-03-13, Barrett Brown: Added clearer BufferManager function comments.
Preconditions:
- Pipeline and buffering config are initialized before use.
- Services submit normalized IngressEvent values through the manager.
Acceptable Input Values/Types:
- Source definitions, ingress events, and pressure callbacks.
Unacceptable Input Values/Types:
- Nil callbacks or invalid buffering config values.
Postconditions:
- Buffers events, updates pressure state, and forwards admitted events into the pipeline.
Return Values/Types:
- Public methods return snapshots, errors, or no value depending on the operation.
Error/Exception Conditions:
- Pipeline publish failures are logged during run loop delivery.
Side Effects:
- Owns shared queue state and signals the dispatcher when work is ready.
Invariants:
- One mutex protects queue state, counters, and active service scheduling.
- Events are only evicted from the same service queue that owns them.
Known Faults:
- All queue work still runs under one mutex, which may cap throughput later.
*/

import (
	"context"
	"log"
	"slices"
	"sync"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
)

const defaultBufferChunkSize = 32

type bufferedEvent struct {
	seq   uint64
	units int
	event IngressEvent
}

type queueChunk struct {
	items []bufferedEvent
	head  int
	tail  int
	next  *queueChunk
}

type serviceQueue struct {
	head *queueChunk
	tail *queueChunk
}

type serviceQueueState struct {
	definition config.SourceDefinition
	queue      serviceQueue
	queueLen   int
	usedUnits  int
	active     bool
	sampleTick uint64
	dropped    uint64
	evicted    uint64
	sampled    uint64
}

type bufferServiceSnapshot struct {
	ReservedUnits int
	BufferedUnits int
	QueueDepth    int
	Sampling      bool
	DroppedEvents uint64
	EvictedEvents uint64
	SampledEvents uint64
}

type bufferSnapshot struct {
	PressureState       PressureState
	TotalBufferedUnits  int
	TotalCapacityUnits  int
	SharedUnitsUsed     int
	SharedUnitsCapacity int
	Services            map[string]bufferServiceSnapshot
}

type BufferManager struct {
	pipeline           *Pipeline
	chunkSize          int
	mu                 sync.Mutex
	runtime            config.BufferingConfig
	services           map[string]*serviceQueueState
	activeServices     []string
	usedUnits          int
	totalReservedUnits int
	sharedUnits        int
	sharedUsedUnits    int
	nextSeq            uint64
	pressureState      PressureState
	pressureCallbacks  []func(PressureState)
	readyCh            chan struct{}
}

// description: Builds a BufferManager with one shared pipeline and buffering policy.
// input: shared Pipeline, BufferingConfig, and optional chunk size override.
// output: Returns an initialized BufferManager ready to accept services and events.
func NewBufferManager(pipeline *Pipeline, runtime config.BufferingConfig, chunkSize int) *BufferManager {
	if chunkSize <= 0 {
		chunkSize = defaultBufferChunkSize
	}
	log.Printf("BufferManager chunk size=%d", chunkSize)
	return &BufferManager{
		pipeline:      pipeline,
		chunkSize:     chunkSize,
		runtime:       runtime,
		services:      make(map[string]*serviceQueueState),
		sharedUnits:   runtime.SharedUnits,
		pressureState: PressureNormal,
		readyCh:       make(chan struct{}, 1),
	}
}

// description: Registers a callback for pressure state transitions.
// input: callback function that accepts the new PressureState.
// output: Stores the callback when it is non nil.
func (m *BufferManager) RegisterPressureCallback(fn func(PressureState)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pressureCallbacks = append(m.pressureCallbacks, fn)
}

// description: Runs the dispatch loop that drains queued events into the shared pipeline.
// input: context used to stop the run loop cleanly.
// output: Returns when the context is canceled.
func (m *BufferManager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.readyCh:
		}

		for {
			next, ok, callbacks, state := m.dequeueNext()
			for _, callback := range callbacks {
				callback(state)
			}
			if !ok {
				break
			}
			if err := m.pipeline.Publish(ctx, next.event); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("BufferManager publish error source=%s: %v", next.event.SourceName, err)
			}
		}
	}
}

// description: Submits one ingress event into the per service buffering path.
// input: source definition and normalized ingress event.
// output: Returns nil after enqueue, sampling, or local drop handling.
func (m *BufferManager) Submit(_ context.Context, definition config.SourceDefinition, event IngressEvent) error {
	callbacks, state, admitted := m.submit(definition, event)
	for _, callback := range callbacks {
		callback(state)
	}
	if admitted {
		m.signalReady()
	}
	return nil
}

func (m *BufferManager) UpsertService(definition config.SourceDefinition) {
	callbacks, state := m.upsertService(definition)
	for _, callback := range callbacks {
		callback(state)
	}
}

func (m *BufferManager) RemoveService(name string) {
	callbacks, state := m.removeService(name)
	for _, callback := range callbacks {
		callback(state)
	}
}

func (m *BufferManager) ApplyRuntimeConfig(runtime config.BufferingConfig) {
	callbacks, state := m.applyRuntimeConfig(runtime)
	for _, callback := range callbacks {
		callback(state)
	}
}

// description: Returns a detached snapshot of current buffering state.
// input: None.
// output: Returns the current bufferSnapshot for status rendering.
func (m *BufferManager) Snapshot() bufferSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked()
}

// description: Performs the locked submit path including sampling and overload policy checks.
// input: source definition and normalized ingress event.
// output: Returns callbacks, resulting pressure state, and whether the event was admitted.
func (m *BufferManager) submit(definition config.SourceDefinition, event IngressEvent) ([]func(PressureState), PressureState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	service := m.ensureServiceLocked(definition)
	// Apply hybrid sampling for non backpressure services under sustained pressure.
	if m.samplingActiveLocked(service) {
		service.sampleTick++
		if service.sampleTick%uint64(maxInt(definition.FlowControl.SamplingEveryN, 1)) != 0 {
			service.sampled++
			return nil, m.pressureState, false
		}
	}

	buffered := bufferedEvent{
		seq:   m.nextSeq,
		units: eventUnits(event),
		event: event,
	}
	m.nextSeq++

	// Admit the event using reserved units first, then shared units, then local eviction.
	admitted := m.admitLocked(service, buffered)
	changed, state, callbacks := m.updatePressureLocked()
	if changed {
		return callbacks, state, admitted
	}
	return nil, m.pressureState, admitted
}

func (m *BufferManager) upsertService(definition config.SourceDefinition) ([]func(PressureState), PressureState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureServiceLocked(definition)
	changed, state, callbacks := m.updatePressureLocked()
	if changed {
		return callbacks, state
	}
	return nil, m.pressureState
}

func (m *BufferManager) removeService(name string) ([]func(PressureState), PressureState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	service := m.services[name]
	if service == nil {
		return nil, m.pressureState
	}

	delete(m.services, name)
	m.activeServices = slices.DeleteFunc(m.activeServices, func(candidate string) bool {
		return candidate == name
	})
	service.active = false

	m.totalReservedUnits -= service.definition.FlowControl.ReservedUnits
	m.usedUnits -= service.usedUnits
	m.sharedUsedUnits -= borrowedUnits(service)
	if m.sharedUsedUnits < 0 {
		m.sharedUsedUnits = 0
	}

	changed, state, callbacks := m.updatePressureLocked()
	if changed {
		return callbacks, state
	}
	return nil, m.pressureState
}

func (m *BufferManager) applyRuntimeConfig(runtime config.BufferingConfig) ([]func(PressureState), PressureState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.runtime = runtime
	m.sharedUnits = runtime.SharedUnits
	changed, state, callbacks := m.updatePressureLocked()
	if changed {
		return callbacks, state
	}
	return nil, m.pressureState
}

// description: Removes the next fair queued event from the active service ring.
// input: None.
// output: Returns the next buffered event if one is available.
func (m *BufferManager) dequeueNext() (*bufferedEvent, bool, []func(PressureState), PressureState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for len(m.activeServices) > 0 {
		name := m.activeServices[0]
		m.activeServices[0] = ""
		m.activeServices = m.activeServices[1:]

		service := m.services[name]
		if service == nil || !service.active || service.queueLen == 0 {
			if service != nil && service.queueLen == 0 {
				service.active = false
			}
			continue
		}

		item, ok := service.queue.popFront()
		if !ok {
			service.active = false
			continue
		}

		service.queueLen--
		m.adjustServiceUsageLocked(service, -item.units)
		if service.queueLen > 0 {
			m.activeServices = append(m.activeServices, name)
		} else {
			service.active = false
		}

		changed, state, callbacks := m.updatePressureLocked()
		if changed {
			return &item, true, callbacks, state
		}
		return &item, true, nil, m.pressureState
	}

	return nil, false, nil, m.pressureState
}

func (m *BufferManager) ensureServiceLocked(definition config.SourceDefinition) *serviceQueueState {
	if service, ok := m.services[definition.Name]; ok {
		m.updateServiceDefinitionLocked(service, definition)
		return service
	}

	service := &serviceQueueState{definition: definition}
	m.services[definition.Name] = service
	m.totalReservedUnits += definition.FlowControl.ReservedUnits
	return service
}

func (m *BufferManager) updateServiceDefinitionLocked(service *serviceQueueState, definition config.SourceDefinition) {
	previousReserved := service.definition.FlowControl.ReservedUnits
	previousBorrowed := borrowedUnits(service)
	service.definition = definition
	m.totalReservedUnits += definition.FlowControl.ReservedUnits - previousReserved
	m.sharedUsedUnits += borrowedUnits(service) - previousBorrowed
}

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
