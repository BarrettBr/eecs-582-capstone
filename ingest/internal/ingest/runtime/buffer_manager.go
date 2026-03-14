package runtime

/*
Name: ingest/internal/ingest/runtime/buffer_manager.go
Description: Buffers ingress events per service, enforces shared unit limits, and dispatches events fairly into the shared pipeline.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added Base logic.
- 2026-03-13, Barrett Brown: Added clearer BufferManager function comments.
- 2026-03-14, Barrett Brown: Split the larger buffer manager into focused modules for runtime flow and queue mechanics.
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
