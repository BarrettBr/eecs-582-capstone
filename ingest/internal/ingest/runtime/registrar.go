package runtime

/*
Name: ingest/internal/ingest/runtime/registrar.go
Description: Owns service lifecycle, hot reloads the source catalog, and merges runtime status snapshots.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-15
Revision History:
- 2026-03-07, Barrett Brown: Added base logic.
- 2026-03-13, Barrett Brown: Added clearer registrar lifecycle comments.
- 2026-03-14, Barrett Brown: Updated registrar reload matching notes to reflect the explicit service fingerprint path.
- 2026-03-14, Barrett Brown: Added catalog-apply callbacks for websocket service pruning and catalog broadcast updates.
- 2026-03-15, Barrett Brown: Merged admitted event totals into the runtime status snapshot for service rate reporting.
Preconditions:
- Source config path, initial catalog, and BufferManager are initialized.
Acceptable Input Values/Types:
- Valid source catalogs, pressure callbacks, and simulator fault injection requests.
Unacceptable Input Values/Types:
- Nil BufferManager or missing source config path.
Postconditions:
- Starts and stops managed services to match the current source catalog.
Return Values/Types:
- Public methods return snapshots, errors, or no value depending on the operation.
Error/Exception Conditions:
- Source reload failures and service startup failures.
Side Effects:
- Starts service goroutines, watches the source config file, and logs lifecycle changes.
Invariants:
- Registrar is the only runtime owner of source service lifecycle.
Known Faults:
- Reload matching still relies on a local managed-service fingerprint helper.
*/

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
)

type RegistrarConfig struct {
	SourceConfigPath      string
	SourceProfile         string
	DefaultModbusInterval time.Duration
	InitialCatalog        *config.SourceCatalog
	BufferManager         *BufferManager
	OnCatalogApplied      func(SystemStatusSnapshot, []string)
}

type Registrar struct {
	sourceConfigPath      string
	sourceProfile         string
	defaultModbusInterval time.Duration
	bufferManager         *BufferManager

	mu               sync.RWMutex
	services         map[string]*managedService
	currentCatalog   *config.SourceCatalog
	currentPressure  PressureState
	lastReloadAt     time.Time
	lastReloadError  string
	onCatalogApplied func(SystemStatusSnapshot, []string)
	runCtx           context.Context
	validationStop   context.CancelFunc
}

type runtimeServicePlan struct {
	InstanceName string
	Definition   config.SourceDefinition
	Identity     managedServiceIdentity
}

// description: Builds a Registrar with source config path, initial catalog, and shared BufferManager.
// input: RegistrarConfig with source path, poll interval, initial catalog, and manager.
// output: Returns an initialized Registrar or an error for missing requirements.
func NewRegistrar(cfg RegistrarConfig) (*Registrar, error) {
	if cfg.SourceConfigPath == "" {
		return nil, fmt.Errorf("registrar requires source config path")
	}
	if cfg.BufferManager == nil {
		return nil, fmt.Errorf("registrar requires buffer manager")
	}

	r := &Registrar{
		sourceConfigPath:      cfg.SourceConfigPath,
		sourceProfile:         cfg.SourceProfile,
		defaultModbusInterval: cfg.DefaultModbusInterval,
		bufferManager:         cfg.BufferManager,
		services:              make(map[string]*managedService),
		currentPressure:       PressureNormal,
		currentCatalog:        cfg.InitialCatalog,
		onCatalogApplied:      cfg.OnCatalogApplied,
	}
	r.bufferManager.RegisterPressureCallback(r.handlePressureChange)
	return r, nil
}

// description: Applies the initial catalog, starts the watcher, and owns service shutdown.
// input: context used to stop the registrar and all managed services.
// output: Returns the context error or a startup failure.
func (r *Registrar) Run(ctx context.Context) error {
	r.mu.Lock()
	r.runCtx = ctx
	r.mu.Unlock()

	if r.currentCatalog == nil {
		return fmt.Errorf("registrar requires initial catalog")
	}
	if err := r.applyCatalog(ctx, r.currentCatalog, true); err != nil {
		return err
	}

	go func() {
		if err := config.WatchSourceCatalog(ctx, r.sourceConfigPath, 200*time.Millisecond, func() error {
			return r.Reload(ctx)
		}); err != nil && ctx.Err() == nil {
			r.setReloadError(err)
			log.Printf("Registrar watch error: %v", err)
		}
	}()

	<-ctx.Done()

	r.mu.Lock()
	if r.validationStop != nil {
		r.validationStop()
		r.validationStop = nil
	}
	r.mu.Unlock()

	r.mu.Lock()
	services := make([]*managedService, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}
	r.mu.Unlock()
	for _, service := range services {
		service.Stop()
	}

	return ctx.Err()
}

// description: Reloads the source catalog from disk and applies the new service set.
// input: context used for any service restarts during reload.
// output: Returns nil when the reload succeeds.
func (r *Registrar) Reload(ctx context.Context) error {
	catalog, err := config.LoadSourceCatalogWithProfile(r.sourceConfigPath, r.sourceProfile)
	if err != nil {
		r.setReloadError(err)
		return err
	}
	if err := r.applyCatalog(ctx, catalog, false); err != nil {
		r.setReloadError(err)
		return err
	}
	return nil
}

// description: Triggers fault injection on one simulator source or the first available one.
// input: optional source name from the control message.
// output: Returns nil when fault injection is enabled on a simulator.
func (r *Registrar) TriggerFaultInjection(sourceName string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if sourceName != "" {
		service, ok := r.services[sourceName]
		if ok {
			return service.TriggerFaultInjection()
		}

		// Support parent service names by routing to the first matching machine instance.
		for _, candidate := range r.services {
			if candidate.identity.ServiceName == sourceName && candidate.definition.Mode == "simulator" {
				return candidate.TriggerFaultInjection()
			}
		}
		return fmt.Errorf("simulator source %q not found", sourceName)
	}

	for _, service := range r.services {
		if service.definition.Mode == "simulator" {
			return service.TriggerFaultInjection()
		}
	}

	return fmt.Errorf("no simulator source available for fault injection")
}

// description: Merges registrar service state with BufferManager counters into one API snapshot.
// input: None.
// output: Returns a detached SystemStatusSnapshot for API use.
func (r *Registrar) StatusSnapshot() SystemStatusSnapshot {
	r.mu.RLock()
	services := make(map[string]ServiceStatus, len(r.services))
	for instanceName, service := range r.services {
		services[instanceName] = service.snapshot()
	}
	lastReloadAt := r.lastReloadAt
	lastReloadError := r.lastReloadError
	r.mu.RUnlock()

	bufferSnapshot := r.bufferManager.Snapshot()
	aggregatedByService := make(map[string]*ServiceStatus, len(services))
	serviceNames := make([]string, 0, len(services))
	for instanceName, status := range services {
		if bufferStatus, ok := bufferSnapshot.Services[instanceName]; ok {
			status.ReservedUnits = bufferStatus.ReservedUnits
			status.BufferedUnits = bufferStatus.BufferedUnits
			status.QueueDepth = bufferStatus.QueueDepth
			status.AdmittedEvents = bufferStatus.AdmittedEvents
			status.Sampling = bufferStatus.Sampling
			status.DroppedEvents = bufferStatus.DroppedEvents
			status.EvictedEvents = bufferStatus.EvictedEvents
			status.SampledEvents = bufferStatus.SampledEvents
		}

		aggregated := aggregatedByService[status.Name]
		if aggregated == nil {
			copied := status
			copied.Machines = nil
			copied.MachineID = ""
			copied.MachineName = ""
			copied.InstanceName = ""
			copied.MachineCount = 0
			aggregatedByService[status.Name] = &copied
			aggregated = &copied
			serviceNames = append(serviceNames, status.Name)
		}

		aggregated.ReservedUnits += status.ReservedUnits
		aggregated.BufferedUnits += status.BufferedUnits
		aggregated.QueueDepth += status.QueueDepth
		aggregated.AdmittedEvents += status.AdmittedEvents
		aggregated.DroppedEvents += status.DroppedEvents
		aggregated.EvictedEvents += status.EvictedEvents
		aggregated.SampledEvents += status.SampledEvents
		aggregated.RecentAlertCount += status.RecentAlertCount
		aggregated.MachineCount++
		aggregated.Connected = aggregated.Connected || status.Connected
		if status.CurrentInterval > aggregated.CurrentInterval {
			aggregated.CurrentInterval = status.CurrentInterval
		}
		if status.LastError != "" {
			aggregated.LastError = status.LastError
		}
		if status.LastAnomalyAt > aggregated.LastAnomalyAt {
			aggregated.LastAnomalyAt = status.LastAnomalyAt
		}
		if status.LifecycleState != "running" && aggregated.LifecycleState == "running" {
			aggregated.LifecycleState = "degraded"
		}
		if status.RateReduced {
			aggregated.RateReduced = true
		}
		if status.Sampling {
			aggregated.Sampling = true
		}

		aggregated.Machines = append(aggregated.Machines, MachineStatus{
			ID:             status.MachineID,
			Name:           status.MachineName,
			ServiceName:    status.Name,
			InstanceName:   instanceName,
			LifecycleState: status.LifecycleState,
			Connected:      status.Connected,
			RecentAlerts:   status.RecentAlertCount,
			AdmittedEvents: status.AdmittedEvents,
			LastAnomalyAt:  status.LastAnomalyAt,
		})
	}
	merged := make([]ServiceStatus, 0, len(aggregatedByService))
	slices.Sort(serviceNames)
	for _, name := range serviceNames {
		service := aggregatedByService[name]
		if service == nil {
			continue
		}
		slices.SortFunc(service.Machines, func(left, right MachineStatus) int {
			return strings.Compare(left.ID, right.ID)
		})
		if service.MachineCount > 0 {
			service.Connected = allMachinesConnected(service.Machines)
		}
		merged = append(merged, *service)
	}

	snapshot := SystemStatusSnapshot{
		PressureState:       bufferSnapshot.PressureState,
		TotalBufferedUnits:  bufferSnapshot.TotalBufferedUnits,
		TotalCapacityUnits:  bufferSnapshot.TotalCapacityUnits,
		SharedUnitsUsed:     bufferSnapshot.SharedUnitsUsed,
		SharedUnitsCapacity: bufferSnapshot.SharedUnitsCapacity,
		Services:            merged,
	}
	if !lastReloadAt.IsZero() {
		snapshot.LastReloadAt = lastReloadAt.UTC().Format(time.RFC3339Nano)
	}
	if lastReloadError != "" {
		snapshot.LastReloadError = lastReloadError
	}
	return snapshot
}

func allMachinesConnected(machines []MachineStatus) bool {
	if len(machines) == 0 {
		return false
	}
	for _, machine := range machines {
		if !machine.Connected {
			return false
		}
	}
	return true
}

// description: Applies pressure changes to all managed services and logs rate reduction changes.
// input: new global PressureState from the BufferManager.
// output: Updates service polling state in place.
func (r *Registrar) handlePressureChange(state PressureState) {
	r.mu.Lock()
	if r.currentPressure == state {
		r.mu.Unlock()
		return
	}
	r.currentPressure = state
	services := make([]*managedService, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}
	r.mu.Unlock()

	log.Printf("Registrar pressure state changed to %s", state)
	for _, service := range services {
		before := service.snapshot().RateReduced
		service.SetPressureState(state)
		after := service.snapshot().RateReduced
		if before != after {
			log.Printf("Service=%s rate_reduced=%t interval=%s", service.definition.Name, after, service.interval())
		}
	}
}

// description: Diffs one source catalog against current services and applies the changes.
// input: context, candidate source catalog, and initial startup flag.
// output: Returns nil when the full catalog apply completes successfully.
func (r *Registrar) applyCatalog(ctx context.Context, catalog *config.SourceCatalog, initial bool) error {
	r.bufferManager.ApplyRuntimeConfig(catalog.Runtime.Buffering)

	plans := expandRuntimeServicePlans(catalog)
	nextDefinitions := make(map[string]runtimeServicePlan, len(plans))
	nextParents := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		nextDefinitions[plan.InstanceName] = plan
		nextParents[plan.Identity.ServiceName] = struct{}{}
	}

	// Start new services or replace changed service definitions.
	for instanceName, plan := range nextDefinitions {
		r.mu.RLock()
		existing := r.services[instanceName]
		currentPressure := r.currentPressure
		r.mu.RUnlock()

		// Reuse the existing service when the definition is unchanged.
		if existing != nil && existing.MatchesDefinition(plan.Definition) {
			r.bufferManager.UpsertService(plan.Definition)
			existing.SetPressureState(currentPressure)
			continue
		}

		// Build a replacement service before disturbing the currently running one.
		replacement, err := newManagedService(plan.Definition, plan.Identity, r.defaultModbusInterval, r.bufferManager)
		if err != nil {
			if existing != nil {
				log.Printf("Registrar kept old service=%s machine=%s after reload prep error: %v", plan.Identity.ServiceName, plan.Identity.MachineID, err)
				continue
			}
			log.Printf("Registrar failed to start new service=%s machine=%s: %v", plan.Identity.ServiceName, plan.Identity.MachineID, err)
			continue
		}

		// Move the service state over to the replacement and retire the old instance.
		replacement.SetPressureState(currentPressure)
		if existing != nil {
			existing.Stop()
		}
		r.bufferManager.UpsertService(plan.Definition)

		// Publish the replacement so future lookups see the new service.
		r.mu.Lock()
		r.services[instanceName] = replacement
		r.mu.Unlock()

		// Start the replacement and report whether it was added or restarted.
		replacement.Start(ctx)
		if existing != nil {
			log.Printf("Registrar restarted service=%s machine=%s", plan.Identity.ServiceName, plan.Identity.MachineID)
		} else if initial {
			log.Printf("Registrar started service=%s machine=%s", plan.Identity.ServiceName, plan.Identity.MachineID)
		} else {
			log.Printf("Registrar added service=%s machine=%s", plan.Identity.ServiceName, plan.Identity.MachineID)
		}
	}

	r.mu.RLock()
	currentServices := make([]string, 0, len(r.services))
	for name := range r.services {
		currentServices = append(currentServices, name)
	}
	r.mu.RUnlock()

	// Stop services that were removed from the latest catalog.
	removedServices := make(map[string]struct{})
	for _, instanceName := range currentServices {
		if _, ok := nextDefinitions[instanceName]; ok {
			continue
		}

		r.mu.Lock()
		service := r.services[instanceName]
		delete(r.services, instanceName)
		r.mu.Unlock()

		if service != nil {
			service.Stop()
		}
		r.bufferManager.RemoveService(instanceName)
		if service != nil && service.identity.ServiceName != "" {
			if _, keep := nextParents[service.identity.ServiceName]; !keep {
				removedServices[service.identity.ServiceName] = struct{}{}
			}
			log.Printf("Registrar removed service=%s machine=%s", service.identity.ServiceName, service.identity.MachineID)
		} else {
			log.Printf("Registrar removed instance=%s", instanceName)
		}
	}

	r.mu.Lock()
	r.currentCatalog = catalog
	r.lastReloadAt = time.Now().UTC()
	r.lastReloadError = ""
	r.mu.Unlock()
	r.restartValidationWatcher(catalog)
	if !initial {
		log.Printf("Registrar applied source reload from %s", r.sourceConfigPath)
	}
	if r.onCatalogApplied != nil {
		r.onCatalogApplied(r.StatusSnapshot(), sortedKeys(removedServices))
	}
	return nil
}

func (r *Registrar) setReloadError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastReloadError = err.Error()
}

func (r *Registrar) restartValidationWatcher(catalog *config.SourceCatalog) {
	r.mu.RLock()
	runCtx := r.runCtx
	r.mu.RUnlock()
	if runCtx == nil || catalog == nil {
		return
	}

	paths := validationPathsFromCatalog(catalog)

	r.mu.Lock()
	if r.validationStop != nil {
		r.validationStop()
		r.validationStop = nil
	}
	if len(paths) == 0 {
		r.mu.Unlock()
		return
	}
	watchCtx, cancel := context.WithCancel(runCtx)
	r.validationStop = cancel
	r.mu.Unlock()

	go func() {
		if err := config.WatchFilesByHash(watchCtx, paths, 200*time.Millisecond, func() error {
			return r.Reload(runCtx)
		}); err != nil && watchCtx.Err() == nil {
			r.setReloadError(err)
			log.Printf("Registrar validation watch error: %v", err)
		}
	}()
}

func validationPathsFromCatalog(catalog *config.SourceCatalog) []string {
	if catalog == nil {
		return nil
	}

	paths := make([]string, 0, len(catalog.Sources))
	seen := make(map[string]struct{}, len(catalog.Sources))
	for _, source := range catalog.Sources {
		if !source.Enabled || source.ValidationFile == "" {
			continue
		}
		if _, ok := seen[source.ValidationFile]; ok {
			continue
		}
		seen[source.ValidationFile] = struct{}{}
		paths = append(paths, source.ValidationFile)
	}
	slices.Sort(paths)
	return paths
}

func expandRuntimeServicePlans(catalog *config.SourceCatalog) []runtimeServicePlan {
	if catalog == nil {
		return nil
	}

	plans := make([]runtimeServicePlan, 0, len(catalog.Sources))
	for _, source := range catalog.Sources {
		if !source.Enabled {
			continue
		}

		if source.Mode == "simulator" && source.Simulator != nil {
			machineCount := source.Simulator.MachineCount
			if machineCount <= 0 {
				machineCount = 3
			}
			for index := 0; index < machineCount; index++ {
				machineID := fmt.Sprintf("m%d", index+1)
				instance := source
				instance.Name = fmt.Sprintf("%s::%s", source.Name, machineID)
				simulatorCopy := *source.Simulator
				simulatorCopy.Seed = source.Simulator.Seed + int64(index)
				instance.Simulator = &simulatorCopy
				plans = append(plans, runtimeServicePlan{
					InstanceName: instance.Name,
					Definition:   instance,
					Identity: managedServiceIdentity{
						ServiceName: source.Name,
						AliasName:   source.AliasName,
						MachineID:   machineID,
						MachineName: machineDisplayName(source, index+1),
					},
				})
			}
			continue
		}

		instance := source
		instance.Name = source.Name
		plans = append(plans, runtimeServicePlan{
			InstanceName: instance.Name,
			Definition:   instance,
			Identity: managedServiceIdentity{
				ServiceName: source.Name,
				AliasName:   source.AliasName,
				MachineID:   "m1",
				MachineName: machineDisplayName(source, 1),
			},
		})
	}

	return plans
}

func machineDisplayName(source config.SourceDefinition, machineNumber int) string {
	base := strings.TrimSpace(source.AliasName)
	if base == "" {
		base = source.Name
	}
	return fmt.Sprintf("%s Machine %d", base, machineNumber)
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
