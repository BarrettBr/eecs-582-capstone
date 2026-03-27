package main

/*
Name: ingest/main.go
Description: Starts the ingest app, loads config, runs DB migrations, and keeps the Modbus loop running.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-03-15
Revision History:
- 2026-02-01, Barrett Brown: Created the file and the base structure
- 2026-02-05, Barrett Brown: Added migrations and sqlc framework
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
- 2026-02-28, Barrett Brown: Added DB to modbus loop for use in fanout
- 2026-03-14, Barrett Brown: Wired registrar catalog updates into websocket service-room broadcasting and pruning.
- 2026-03-15, Barrett Brown: Added aggregated websocket service rate updates built from admitted event totals.
Preconditions:
- Env vars are available (or defaults can be used).
- DB file path and migration files can be accessed.
Acceptable Input Values/Types:
- String env var values for config.
- Normal shutdown signals like Ctrl+C/SIGTERM.
Unacceptable Input Values/Types:
- Bad config values (like invalid duration or register number).
- Missing DB/migration paths.
Postconditions:
- Service runs and polls Modbus, or exits with an error.
Return Values/Types:
- main: no return value.
- runMigrations: error (nil = success, non-nil = failure).
Error/Exception Conditions:
- Config, DB, migration, or ingest startup/runtime failures.
Side Effects:
- Opens DB, runs migrations, starts polling loop, writes logs.
Invariants:
- Ingest loop only starts after config and migrations succeed.
Known Faults:
- Uses fatal exits instead of trying to recover.
*/

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/api"
	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	ingestruntime "github.com/BarrettBr/eecs-582-capstone/internal/ingest/runtime"
	"github.com/BarrettBr/eecs-582-capstone/internal/stream"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// description: Bootstraps config, runs DB migrations, and starts the ingest loop until shutdown.
// input: None (reads runtime configuration from environment and defaults).
// output: None (process exits on fatal startup/runtime errors).
func main() {
	// Load .env if it exists.
	_ = godotenv.Load()

	appCfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error building config: %v", err)
	}
	defer appCfg.DB.Close()

	// Run database migrations
	if err := runMigrations(appCfg.DB, appCfg.Database.MigrationsPath); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	wsServer := stream.NewServer(stream.Config{
		Addr:               appCfg.Stream.Address,
		Path:               appCfg.Stream.Path,
		BatchSize:          appCfg.Stream.BatchSize,
		BatchFlushInterval: appCfg.Stream.BatchFlushInterval,
	})
	pipeline, err := ingestruntime.NewPipeline(ingestruntime.PipelineDependencies{
		DB:       appCfg.DB,
		Queries:  appCfg.Queries,
		Streamer: wsServer,
	}, ingestruntime.PipelineConfig{
		MLFanout:  appCfg.Ingest.MLFanout,
		SQLFanout: appCfg.Ingest.SQLFanout,
	})
	if err != nil {
		log.Fatalf("Error creating ingest pipeline: %v", err)
	}
	bufferManager := ingestruntime.NewBufferManager(pipeline, appCfg.Sources.Runtime.Buffering, appCfg.Ingest.BufferChunkSize)
	registrar, err := ingestruntime.NewRegistrar(ingestruntime.RegistrarConfig{
		SourceConfigPath:      appCfg.Ingest.SourceConfigPath,
		SourceProfile:         appCfg.Ingest.SourceProfile,
		DefaultModbusInterval: appCfg.Ingest.PollInterval,
		InitialCatalog:        appCfg.Sources,
		BufferManager:         bufferManager,
		OnCatalogApplied: func(snapshot ingestruntime.SystemStatusSnapshot, removed []string) {
			wsServer.ApplyServiceCatalog(buildServiceCatalogPayload(snapshot), removed)
		},
	})
	if err != nil {
		log.Fatalf("Error creating registrar: %v", err)
	}

	type faultInjectRequest struct {
		SourceName string `json:"source_name"`
	}
	wsServer.RegisterReadHandler("inject_fault", func(_ context.Context, _ stream.ClientSession, msg stream.Message) {
		var req faultInjectRequest
		if len(msg.Data) > 0 {
			if err := json.Unmarshal(msg.Data, &req); err != nil {
				log.Printf("Fault injection decode error: %v", err)
				return
			}
		}
		if err := registrar.TriggerFaultInjection(req.SourceName); err != nil {
			log.Printf("Fault injection request failed: %v", err)
			return
		}
		log.Printf("Fault injection triggered for source=%q", req.SourceName)
	})
	apiRegistration := api.RegisterRoutes(wsServer, api.Config{
		BasePath:         appCfg.Stream.APIBasePath,
		Queries:          appCfg.Queries,
		Status:           registrar,
		SourceConfigPath: appCfg.Ingest.SourceConfigPath,
		SourceProfile:    appCfg.Ingest.SourceProfile,
	})

	go func() {
		if err := wsServer.Start(ctx); err != nil {
			log.Printf("Websocket server error: %v", err)
		}
	}()
	go pipeline.Run(ctx)
	go bufferManager.Run(ctx)
	go broadcastServiceRates(ctx, wsServer, registrar, 5*time.Second)

	log.Printf("Ingest registrar ready. Default Modbus poll interval: %s", appCfg.Ingest.PollInterval)
	log.Printf("API ready at http://%s%s", appCfg.Stream.Address, apiRegistration.PingPath)
	log.Printf("Websocket stream ready at %s", wsServer.URL())
	if err := registrar.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Error running ingest registrar: %v", err)
	}
}

// description: Applies goose migrations to the configured SQLite database.
// input: db (open SQL connection), migrationsPath (filesystem path to migration files).
// output: Returns nil on success, or an error if dialect setup/migration fails.
func runMigrations(db *sql.DB, migrationsPath string) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("Error setting goose dialect: %w", err)
	}
	if err := goose.Up(db, migrationsPath); err != nil {
		return fmt.Errorf("Error, goose up: %w", err)
	}
	return nil
}

func buildServiceCatalogPayload(snapshot ingestruntime.SystemStatusSnapshot) stream.ServiceCatalogPayload {
	services := make([]stream.ServiceCatalogEntry, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		services = append(services, stream.ServiceCatalogEntry{
			Name:      service.Name,
			AliasName: service.AliasName,
			Mode:      service.Mode,
			EventType: service.EventType,
		})
	}
	return stream.ServiceCatalogPayload{
		Revision: snapshot.LastReloadAt,
		Services: services,
	}
}

type serviceRateEntry struct {
	Name          string  `json:"name"`
	AdmittedEPS5s float64 `json:"admitted_eps_5s"`
}

type serviceRatesPayload struct {
	IntervalSeconds int                `json:"interval_seconds"`
	Services        []serviceRateEntry `json:"services"`
}

func broadcastServiceRates(ctx context.Context, wsServer *stream.Server, registrar *ingestruntime.Registrar, interval time.Duration) {
	if wsServer == nil || registrar == nil || interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	previousTotals := make(map[string]uint64)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := registrar.StatusSnapshot()
			payload := buildServiceRatesPayload(snapshot, previousTotals, interval)
			previousTotals = snapshotServiceTotals(snapshot)
			data, err := json.Marshal(payload)
			if err != nil {
				log.Printf("Service rate payload encode error: %v", err)
				continue
			}
			wsServer.Publish(stream.Message{
				Kind:      "service_rates",
				Source:    "stream",
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Data:      data,
			})
		}
	}
}

func buildServiceRatesPayload(snapshot ingestruntime.SystemStatusSnapshot, previousTotals map[string]uint64, interval time.Duration) serviceRatesPayload {
	intervalSeconds := interval.Seconds()
	if intervalSeconds <= 0 {
		intervalSeconds = 1
	}

	services := make([]serviceRateEntry, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		previous := previousTotals[service.Name]
		delta := service.AdmittedEvents - previous
		rate := float64(delta) / intervalSeconds
		services = append(services, serviceRateEntry{
			Name:          service.Name,
			AdmittedEPS5s: roundRate(rate),
		})
	}

	return serviceRatesPayload{
		IntervalSeconds: int(math.Round(intervalSeconds)),
		Services:        services,
	}
}

func snapshotServiceTotals(snapshot ingestruntime.SystemStatusSnapshot) map[string]uint64 {
	totals := make(map[string]uint64, len(snapshot.Services))
	for _, service := range snapshot.Services {
		totals[service.Name] = service.AdmittedEvents
	}
	return totals
}

func roundRate(rate float64) float64 {
	if rate < 0 {
		return 0
	}
	return math.Round(rate*10) / 10
}
