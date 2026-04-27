package runtime

/*
Name: ingest/internal/ingest/runtime/pipeline.go
Description: Owns the shared downstream fanout queues and dependencies for ML, SQL, and websocket delivery.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer pipeline setup and publish comments.
- 2026-03-14, Barrett Brown: Updated pipeline event contracts during the ingest file structure refactor to use the explicit events package.
Preconditions:
- Database, query, and optional stream dependencies are initialized.
Acceptable Input Values/Types:
- Pipeline dependencies, fanout config values, and normalized ingress events.
Unacceptable Input Values/Types:
- Nil required database dependencies.
Postconditions:
- Provides one shared downstream publish path for all ingest services.
Return Values/Types:
- NewPipeline: (*Pipeline, error)
- Publish returns an error when enqueueing is canceled.
Error/Exception Conditions:
- Missing database dependencies or canceled publish contexts.
Side Effects:
- Creates bounded fanout channels for downstream workers.
Invariants:
- All services share the same downstream pipeline instance.
Known Faults:
- Fanout backpressure is still bounded by in memory channels only.
*/

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
	"github.com/BarrettBr/eecs-582-capstone/internal/stream"
)

// IngressEvent is the normalized event envelope shared across services, buffering, and fanout.
type IngressEvent struct {
	SourceName  string
	ServiceName string
	MachineID   string
	MachineName string
	MLEnabled   bool
	Record      ingestevents.RecordEvent
}

type PipelineDependencies struct {
	DB       *sql.DB
	Queries  *database.Queries
	Streamer *stream.Server
}

type PipelineConfig struct {
	MLFanout  config.MLFanoutConfig
	SQLFanout config.SQLFanoutConfig
}

// Pipeline owns the shared downstream ML, SQL, and websocket fanout workers.
type Pipeline struct {
	db                    *sql.DB
	queries               *database.Queries
	mlCh                  chan IngressEvent
	sqlCh                 chan IngressEvent
	wsCh                  chan IngressEvent
	mlAPIURL              string
	mlHTTP                *http.Client
	mlBatchSize           int
	mlBatchFlushInterval  time.Duration
	mlLastResponse        []byte
	streamer              *stream.Server
	sqlBatchSize          int
	sqlBatchFlushInterval time.Duration
	sqlNormalSampleRate   int
	sqlNormalSampleCount  map[string]int
}

// description: Builds the shared downstream pipeline and applies default fanout settings.
// input: initialized dependencies plus ML and SQL fanout config values.
// output: Returns a ready Pipeline or an error if required dependencies are missing.
func NewPipeline(deps PipelineDependencies, cfg PipelineConfig) (*Pipeline, error) {
	if deps.DB == nil || deps.Queries == nil {
		return nil, fmt.Errorf("pipeline requires database dependencies")
	}
	if cfg.SQLFanout.BatchSize <= 0 {
		cfg.SQLFanout.BatchSize = 1
	}
	if cfg.SQLFanout.BatchFlushInterval <= 0 {
		cfg.SQLFanout.BatchFlushInterval = 25 * time.Millisecond
	}
	if cfg.SQLFanout.NormalSampleRate <= 0 {
		cfg.SQLFanout.NormalSampleRate = 1
	}
	if cfg.MLFanout.BatchSize <= 0 {
		cfg.MLFanout.BatchSize = 1
	}
	if cfg.MLFanout.BatchFlushInterval <= 0 {
		cfg.MLFanout.BatchFlushInterval = 25 * time.Millisecond
	}
	if cfg.MLFanout.HTTPTimeout <= 0 {
		cfg.MLFanout.HTTPTimeout = 500 * time.Millisecond
	}

	return &Pipeline{
		db:       deps.DB,
		queries:  deps.Queries,
		mlCh:     make(chan IngressEvent, 128),
		sqlCh:    make(chan IngressEvent, 128),
		wsCh:     make(chan IngressEvent, 128),
		mlAPIURL: cfg.MLFanout.APIURL,
		mlHTTP: &http.Client{
			Timeout: cfg.MLFanout.HTTPTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				MaxConnsPerHost:     0,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		mlBatchSize:           cfg.MLFanout.BatchSize,
		mlBatchFlushInterval:  cfg.MLFanout.BatchFlushInterval,
		streamer:              deps.Streamer,
		sqlBatchSize:          cfg.SQLFanout.BatchSize,
		sqlBatchFlushInterval: cfg.SQLFanout.BatchFlushInterval,
		sqlNormalSampleRate:   cfg.SQLFanout.NormalSampleRate,
		sqlNormalSampleCount:  make(map[string]int),
	}, nil
}

// description: Starts downstream sink workers and waits for shutdown.
// input: context used to stop the worker set cleanly.
// output: Returns when the context is canceled.
func (p *Pipeline) Run(ctx context.Context) {
	p.startWorkers(ctx)
	<-ctx.Done()
}

// description: Fans one ingress event out to the enabled downstream queues.
// input: context for cancellation plus one normalized IngressEvent.
// output: Returns an error if enqueueing is interrupted by context cancellation.
func (p *Pipeline) Publish(ctx context.Context, event IngressEvent) error {
	if p.mlAPIURL != "" && event.MLEnabled {
		if err := p.enqueue(ctx, "ml", p.mlCh, event); err != nil {
			return err
		}
	}
	if err := p.enqueue(ctx, "sql", p.sqlCh, event); err != nil {
		return err
	}
	if err := p.enqueue(ctx, "websocket", p.wsCh, event); err != nil {
		return err
	}
	return nil
}

func (p *Pipeline) LastMLResponse() []byte {
	return append([]byte(nil), p.mlLastResponse...)
}
