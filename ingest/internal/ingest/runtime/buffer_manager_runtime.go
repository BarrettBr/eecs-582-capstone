package runtime

/*
Name: ingest/internal/ingest/runtime/buffer_manager_runtime.go
Description: Locked BufferManager state transitions for submit, service lifecycle, and fair dequeue behavior.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added Base logic.
- 2026-03-13, Barrett Brown: Added clearer BufferManager function comments.
- 2026-03-14, Barrett Brown: Split the larger buffer manager into focused modules for runtime flow and queue mechanics.
Preconditions:
- Callers use the public BufferManager methods so lock ownership stays internal.
Postconditions:
- Service state and pressure transitions stay consistent after each mutation.
Known Faults:
- Fair dispatch still uses one shared active service slice under the manager mutex.
*/

import (
	"slices"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
)

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
