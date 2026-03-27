package runtime

/*
Name: ingest/internal/ingest/runtime/service_factory.go
Description: Builds source runner instances for simulator and Modbus ingest services.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer source runner setup comments.
- 2026-03-14, Barrett Brown: Updated source runner setup during the ingest file structure refactor to use the explicit validation package.
Preconditions:
- Source definitions are validated before runner construction.
Acceptable Input Values/Types:
- Simulator and modbus source definitions with valid intervals and connection settings.
Unacceptable Input Values/Types:
- Unsupported source modes or invalid interval strings.
Postconditions:
- Returns a sourceRunner configured for the requested source definition.
Return Values/Types:
- newSourceRunner: (*sourceRunner, error)
- close: no return value
Error/Exception Conditions:
- Validation rule load failures, interval parse failures, and Modbus connection failures.
Side Effects:
- Opens Modbus network connections when required.
Invariants:
- Runner interval always reflects either the source override or the default poll interval.
Known Faults:
- Runner construction still mixes validation setup and transport setup in one function.
*/

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	ingestvalidation "github.com/BarrettBr/eecs-582-capstone/internal/ingest/validation"
	"github.com/goburrow/modbus"
)

// description: Builds one sourceRunner from a validated source definition.
// input: source definition and default Modbus poll interval.
// output: Returns a ready sourceRunner or an error.
func newSourceRunner(definition config.SourceDefinition, defaultModbusInterval time.Duration) (*sourceRunner, error) {
	validator, err := ingestvalidation.NewRuleEngine(definition.ValidationFile)
	if err != nil {
		return nil, fmt.Errorf("load validator for %q: %w", definition.Name, err)
	}

	runner := &sourceRunner{
		definition: definition,
		validator:  validator,
	}

	switch definition.Mode {
	case "modbus":
		handler := modbus.NewTCPClientHandler(definition.Modbus.Address)
		handler.Timeout = 10 * time.Second
		handler.SlaveId = definition.Modbus.SlaveID
		if err := handler.Connect(); err != nil {
			return nil, fmt.Errorf("connect source %q (%s): %w", definition.Name, definition.Modbus.Address, err)
		}
		runner.client = modbus.NewClient(handler)
		runner.modbusHandler = handler
		runner.interval = defaultModbusInterval
		if definition.Modbus.Interval != "" {
			d, err := time.ParseDuration(definition.Modbus.Interval)
			if err != nil {
				handler.Close()
				return nil, fmt.Errorf("parse modbus interval for %q: %w", definition.Name, err)
			}
			runner.interval = d
		}
	case "simulator":
		d, err := time.ParseDuration(definition.Simulator.Interval)
		if err != nil {
			return nil, fmt.Errorf("parse simulator interval for %q: %w", definition.Name, err)
		}
		runner.interval = d
		runner.rng = rand.New(rand.NewSource(definition.Simulator.Seed))
		runner.simTemp = 68.0
		runner.simValveFlow = 145.0
		runner.simValveOpen = true
	default:
		return nil, fmt.Errorf("unsupported mode %q", definition.Mode)
	}

	return runner, nil
}

func (s *sourceRunner) close() {
	if s == nil || s.modbusHandler == nil {
		return
	}
	_ = s.modbusHandler.Close()
}
