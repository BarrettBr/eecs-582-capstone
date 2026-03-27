package runtime

/*
Name: ingest/internal/ingest/runtime/service.go
Description: Runs one managed ingest service under registrar control and submits normalized events into the BufferManager.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-15
Revision History:
- 2026-03-07, Barrett Brown: Added Base logic.
- 2026-03-13, Barrett Brown: Added clearer managed service lifecycle comments.
- 2026-03-14, Barrett Brown: Replaced reflect-based definition matching with an explicit normalized fingerprint.
- 2026-03-15, Barrett Brown: Switched managed service scheduling from fixed-delay to fixed-rate deadline scheduling.
Preconditions:
- Managed services are created from validated source definitions.
- Source runners and submitters are initialized before service start.
Acceptable Input Values/Types:
- Source definitions, shared context values, and pressure updates.
Unacceptable Input Values/Types:
- Unsupported source definitions or nil submitters.
Postconditions:
- Managed services poll their source, validate records, and submit ingress events while running.
Return Values/Types:
- Constructor and control helpers return pointers, errors, or no value depending on the method.
Error/Exception Conditions:
- Runner creation failures, source read failures, and validation failures.
Side Effects:
- Starts polling goroutines and updates service lifecycle state.
Invariants:
- Each managed service owns one runner and one current polling interval.
Known Faults:
- Definition matching still fingerprints full service config inline instead of using a shared config hash helper.
*/

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
)

type bufferSubmitter interface {
	Submit(context.Context, config.SourceDefinition, IngressEvent) error
}

type managedService struct {
	definition      config.SourceDefinition
	runner          *sourceRunner
	submitter       bufferSubmitter
	baseInterval    time.Duration
	definitionKey   string
	validationKey   string
	mu              sync.RWMutex
	lifecycleState  string
	lastError       string
	pressureState   PressureState
	currentInterval time.Duration
	cancel          context.CancelFunc
	done            chan struct{}
}

// description: Builds one managed service from a validated source definition.
// input: source definition, default Modbus interval, and shared BufferManager submitter.
// output: Returns a managedService ready to start or an error.
func newManagedService(definition config.SourceDefinition, defaultModbusInterval time.Duration, submitter bufferSubmitter) (*managedService, error) {
	runner, err := newSourceRunner(definition, defaultModbusInterval)
	if err != nil {
		return nil, err
	}

	return &managedService{
		definition:      definition,
		runner:          runner,
		submitter:       submitter,
		baseInterval:    runner.interval,
		definitionKey:   sourceDefinitionFingerprint(definition),
		validationKey:   config.FileContentSignature(definition.ValidationFile),
		lifecycleState:  "stopped",
		pressureState:   PressureNormal,
		currentInterval: runner.interval,
		done:            make(chan struct{}),
	}, nil
}

// description: Starts the service polling loop if it is not already running.
// input: parent context used to derive the service run context.
// output: Launches the service goroutine and returns immediately.
func (s *managedService) Start(parent context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.lifecycleState = "starting"
	s.lastError = ""
	s.mu.Unlock()

	go s.run(ctx)
}

// description: Stops the service polling loop and waits for it to exit.
// input: None.
// output: Returns after the service goroutine has stopped.
func (s *managedService) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	if cancel == nil {
		s.lifecycleState = "stopped"
		s.mu.Unlock()
		s.runner.close()
		return
	}
	s.lifecycleState = "stopping"
	s.mu.Unlock()

	cancel()
	<-done
}

// description: Owns the service lifecycle loop and polls the source on a fixed-rate deadline.
// input: service run context.
// output: Returns when the context is canceled.
func (s *managedService) run(ctx context.Context) {
	defer close(s.done)
	defer s.runner.close()
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		if s.lifecycleState != "failed" {
			s.lifecycleState = "stopped"
		}
		s.mu.Unlock()
	}()

	s.mu.Lock()
	s.lifecycleState = "running"
	s.mu.Unlock()

	nextRun := time.Now()
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := s.handleTick(ctx); err != nil && !isContextCanceled(err) {
				s.recordError(err)
				log.Printf("Service=%s tick failed: %v", s.definition.Name, err)
			}
			nextRun = nextFixedRateDeadline(nextRun, time.Now(), s.interval())
			delay := time.Until(nextRun)
			if delay < 0 {
				delay = 0
			}
			timer.Reset(delay)
		}
	}
}

// description: Reads one record, validates it, attaches anomalies, and submits it downstream.
// input: context used for submit cancellation.
// output: Returns nil when the full tick succeeds.
func (s *managedService) handleTick(ctx context.Context) error {
	record, err := s.runner.nextRecord()
	if err != nil {
		return err
	}

	validation := s.runner.validator.Validate(record)
	if !validation.Valid {
		return fmt.Errorf("validation failed: %v", validation.Errors)
	}

	record, err = applyAnomalies(record, validation.Anomalies)
	if err != nil {
		return err
	}

	return s.submitter.Submit(ctx, s.definition, IngressEvent{
		SourceName: s.definition.Name,
		MLEnabled:  s.definition.MLEnabled,
		Record:     record,
	})
}

// description: Updates the service polling interval based on the current pressure state.
// input: new global PressureState value.
// output: Stores the active interval for later polling ticks.
func (s *managedService) SetPressureState(state PressureState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pressureState = state
	s.currentInterval = s.baseInterval
	if !s.definition.FlowControl.AdaptiveEnabledValue() || !s.definition.FlowControl.SupportsBackpressureValue(s.definition.Mode) {
		return
	}

	switch state {
	case PressureHigh:
		s.currentInterval = s.baseInterval * time.Duration(s.definition.FlowControl.Backpressure.HighIntervalMultiplier)
	case PressureCritical:
		s.currentInterval = s.baseInterval * time.Duration(s.definition.FlowControl.Backpressure.CriticalIntervalMultiplier)
	}
}

func (s *managedService) TriggerFaultInjection() error {
	if s.runner == nil {
		return fmt.Errorf("service %q runner unavailable for fault injection", s.definition.Name)
	}
	return s.runner.triggerFaultInjection()
}

func (s *managedService) MatchesDefinition(definition config.SourceDefinition) bool {
	return s.definitionKey == sourceDefinitionFingerprint(definition) &&
		s.validationKey == config.FileContentSignature(definition.ValidationFile)
}

func (s *managedService) interval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentInterval
}

func (s *managedService) snapshot() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ServiceStatus{
		Name:                 s.definition.Name,
		AliasName:            s.definition.AliasName,
		Mode:                 s.definition.Mode,
		EventType:            s.definition.EventType,
		LifecycleState:       s.lifecycleState,
		AdaptiveEnabled:      s.definition.FlowControl.AdaptiveEnabledValue(),
		SupportsBackpressure: s.definition.FlowControl.SupportsBackpressureValue(s.definition.Mode),
		RateReduced:          s.currentInterval > s.baseInterval,
		CurrentInterval:      s.currentInterval,
		LastError:            s.lastError,
	}
}

func (s *managedService) recordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err.Error()
	if s.lifecycleState == "starting" {
		s.lifecycleState = "failed"
	}
}

func nextFixedRateDeadline(previous time.Time, now time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		return now
	}
	if previous.IsZero() {
		return now.Add(interval)
	}

	next := previous.Add(interval)
	if next.After(now) {
		return next
	}

	missedIntervals := now.Sub(next)/interval + 1
	return next.Add(missedIntervals * interval)
}

func sourceDefinitionFingerprint(definition config.SourceDefinition) string {
	var builder strings.Builder
	builder.Grow(256)

	appendStringFingerprintPart(&builder, definition.Name)
	appendStringFingerprintPart(&builder, definition.AliasName)
	appendStringFingerprintPart(&builder, definition.EventType)
	appendStringFingerprintPart(&builder, definition.Mode)
	appendStringFingerprintPart(&builder, definition.ValidationFile)
	appendBoolFingerprintPart(&builder, definition.MLEnabled)

	appendIntFingerprintPart(&builder, definition.FlowControl.ReservedUnits)
	appendBoolFingerprintPart(&builder, definition.FlowControl.AdaptiveEnabledValue())
	appendBoolFingerprintPart(&builder, definition.FlowControl.SupportsBackpressureValue(definition.Mode))
	appendStringFingerprintPart(&builder, definition.FlowControl.OverloadPolicy)
	appendIntFingerprintPart(&builder, definition.FlowControl.SamplingEveryN)
	appendIntFingerprintPart(&builder, definition.FlowControl.Backpressure.HighIntervalMultiplier)
	appendIntFingerprintPart(&builder, definition.FlowControl.Backpressure.CriticalIntervalMultiplier)

	if definition.Modbus == nil {
		appendStringFingerprintPart(&builder, "")
		appendUint64FingerprintPart(&builder, 0)
		appendUint64FingerprintPart(&builder, 0)
		appendUint64FingerprintPart(&builder, 0)
		appendStringFingerprintPart(&builder, "")
	} else {
		appendStringFingerprintPart(&builder, definition.Modbus.Address)
		appendUint64FingerprintPart(&builder, uint64(definition.Modbus.MWBase))
		appendUint64FingerprintPart(&builder, uint64(definition.Modbus.RegisterCount))
		appendUint64FingerprintPart(&builder, uint64(definition.Modbus.SlaveID))
		appendStringFingerprintPart(&builder, definition.Modbus.Interval)
	}

	if definition.Simulator == nil {
		appendStringFingerprintPart(&builder, "")
		appendInt64FingerprintPart(&builder, 0)
		appendStringFingerprintPart(&builder, "")
	} else {
		appendStringFingerprintPart(&builder, definition.Simulator.Interval)
		appendInt64FingerprintPart(&builder, definition.Simulator.Seed)
		appendStringFingerprintPart(&builder, definition.Simulator.Profile)
	}

	return builder.String()
}

func appendStringFingerprintPart(builder *strings.Builder, value string) {
	builder.WriteString(value)
	builder.WriteByte(0)
}

func appendBoolFingerprintPart(builder *strings.Builder, value bool) {
	if value {
		builder.WriteByte('1')
	} else {
		builder.WriteByte('0')
	}
	builder.WriteByte(0)
}

func appendIntFingerprintPart(builder *strings.Builder, value int) {
	builder.WriteString(strconv.Itoa(value))
	builder.WriteByte(0)
}

func appendInt64FingerprintPart(builder *strings.Builder, value int64) {
	builder.WriteString(strconv.FormatInt(value, 10))
	builder.WriteByte(0)
}

func appendUint64FingerprintPart(builder *strings.Builder, value uint64) {
	builder.WriteString(strconv.FormatUint(value, 10))
	builder.WriteByte(0)
}
