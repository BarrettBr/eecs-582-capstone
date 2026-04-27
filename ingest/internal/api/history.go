package api

/*
Name: ingest/internal/api/history.go
Description: Serves history and report API endpoints by mapping request paths to prebuilt database queries.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-13
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer handler and query flow comments.
Preconditions:
- API routes are registered with a live query service.
- Request path values use the expected kind and window names.
Acceptable Input Values/Types:
- HTTP GET requests for history and report endpoints.
- Supported source kinds like temp and valve.
Unacceptable Input Values/Types:
- Unsupported endpoint kinds or windows.
- Nil query dependencies.
Postconditions:
- Returns history item lists or report summaries as JSON responses.
Return Values/Types:
- Handler methods write HTTP responses directly.
- Helper functions return querySelection values or errors.
Error/Exception Conditions:
- Bad path values return 400 responses.
- Query failures return 500 responses.
Side Effects:
- Reads from the database and writes HTTP responses.
Invariants:
- Selection building stays consistent across history and report endpoints.
- Response shapes remain stable for the frontend.
Known Faults:
- History formatting still does per row mapping work in memory.
*/

import (
	"net/http"
	"strings"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

type querySelection struct {
	Endpoint string `json:"endpoint"`
	Kind     string `json:"kind"`
	Window   string `json:"window"`
	Table    string `json:"table"`
	GroupBy  string `json:"group_by"`
	Range    string `json:"range"`
	Seconds  int64  `json:"-"`
}

// description: Handles history endpoint requests and validates the path selection.
// input: HTTP response writer and request with kind and window path values.
// output: Writes a JSON history response or an API error response.
func (cfg *apiConfig) historyHandler(w http.ResponseWriter, r *http.Request) {
	selection, err := buildQuerySelection("history", r.PathValue("kind"), r.PathValue("window"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	cfg.runSelectedQuery(w, r, selection)
}

// description: Handles report endpoint requests and validates the path selection.
// input: HTTP response writer and request with kind and window path values.
// output: Writes a JSON report response or an API error response.
func (cfg *apiConfig) reportHandler(w http.ResponseWriter, r *http.Request) {
	selection, err := buildQuerySelection("report", r.PathValue("kind"), r.PathValue("window"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	cfg.runSelectedQuery(w, r, selection)
}

// description: Converts endpoint path values into one validated query selection.
// input: endpoint name plus kind and window strings from the route.
// output: Returns a filled querySelection or an error for invalid input.
func buildQuerySelection(endpoint, kind, window string) (querySelection, error) {
	selection := querySelection{
		Endpoint: endpoint,
		Kind:     kind,
		Window:   window,
	}

	switch kind {
	case "temp":
		selection.Table = "temp_samples"
	case "valve":
		selection.Table = "valve_samples"
	default:
		return querySelection{}, errorString("invalid " + endpoint + " kind")
	}

	switch window {
	case "hour":
		selection.GroupBy = "minute"
		selection.Range = "1h"
		selection.Seconds = 60 * 60
	case "day":
		selection.GroupBy = "hour"
		selection.Range = "24h"
		selection.Seconds = 24 * 60 * 60
	case "week":
		selection.GroupBy = "day"
		selection.Range = "7d"
		selection.Seconds = 7 * 24 * 60 * 60
	default:
		return querySelection{}, errorString("invalid " + endpoint + " window")
	}

	return selection, nil
}

type historyEvent struct {
	ID          int64    `json:"id"`
	ServiceName string   `json:"service_name,omitempty"`
	MachineID   string   `json:"machine_id,omitempty"`
	Timestamp   int64    `json:"timestamp"`
	SensorType  string   `json:"sensor_type"`
	SourceKind  string   `json:"source_kind"`
	SourceID    int64    `json:"source_id"`
	FanOn       bool     `json:"fan_on,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	HeaterPower float64  `json:"heater_power,omitempty"`
	IsOpen      bool     `json:"is_open,omitempty"`
	FlowRate    float64  `json:"flow_rate,omitempty"`
	AlertCount  int64    `json:"alert_count"`
	Anomalies   []string `json:"anomalies"`
}

type reportSummary struct {
	AvgTemp                  float64 `json:"avg_temp"`
	MinTemp                  float64 `json:"min_temp"`
	MaxTemp                  float64 `json:"max_temp"`
	AvgValveOpenRate         float64 `json:"avg_valve_open_rate"`
	MinValveOpenRate         float64 `json:"min_valve_open_rate"`
	MaxValveOpenRate         float64 `json:"max_valve_open_rate"`
	TotalAlerts              int64   `json:"total_alerts"`
	TempAnomalies            int64   `json:"temp_anomalies"`
	ValveAnomalies           int64   `json:"valve_anomalies"`
	ValveOpenDurationSeconds int64   `json:"valve_open_duration_seconds"`
}

// description: Runs the selected history or report query and formats the JSON response.
// input: HTTP response writer, request context, and a validated querySelection.
// output: Writes a report summary or history item list to the response.
func (cfg *apiConfig) runSelectedQuery(w http.ResponseWriter, r *http.Request, selection querySelection) {
	ctx := r.Context()
	sinceUnix := time.Now().UTC().Unix() - selection.Seconds
	nowUnix := time.Now().UTC().Unix()

	// Run the report query path.
	if selection.Endpoint == "report" {
		summary, err := cfg.queries.GetReportSummaryBetweenUnix(ctx, database.GetReportSummaryBetweenUnixParams{
			SinceUnix: sinceUnix,
			NowUnix:   nowUnix,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed report query", err)
			return
		}
		respondWithJSON(w, http.StatusOK, struct {
			Selection querySelection `json:"selection"`
			Summary   reportSummary  `json:"summary"`
		}{
			Selection: selection,
			Summary: reportSummary{
				AvgTemp:                  summary.AvgTemp,
				MinTemp:                  summary.MinTemp,
				MaxTemp:                  summary.MaxTemp,
				AvgValveOpenRate:         summary.AvgValveOpenRate,
				MinValveOpenRate:         summary.MinValveOpenRate,
				MaxValveOpenRate:         summary.MaxValveOpenRate,
				TotalAlerts:              summary.TotalAlerts,
				TempAnomalies:            summary.TempAnomalies,
				ValveAnomalies:           summary.ValveAnomalies,
				ValveOpenDurationSeconds: summary.ValveOpenDurationSeconds,
			},
		})
		return
	}

	// Build history items from the selected source table.
	items := make([]historyEvent, 0, 256)
	switch selection.Kind {
	case "temp":
		rows, err := cfg.queries.GetTempHistorySinceUnix(ctx, sinceUnix)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed history query", err)
			return
		}
		items = make([]historyEvent, 0, len(rows))
		for _, row := range rows {
			items = append(items, historyEvent{
				ID:          row.ID,
				ServiceName: row.ServiceName,
				MachineID:   row.MachineID,
				Timestamp:   row.Timestamp,
				SensorType:  row.SensorType,
				SourceKind:  "temp",
				SourceID:    row.SensorNumber,
				FanOn:       row.FanOn,
				Temperature: row.Temperature,
				HeaterPower: row.HeaterPower,
				AlertCount:  row.AlertCount,
				Anomalies:   splitAnomalies(row.AnomalyLabels),
			})
		}
	case "valve":
		rows, err := cfg.queries.GetValveHistorySinceUnix(ctx, sinceUnix)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed history query", err)
			return
		}
		items = make([]historyEvent, 0, len(rows))
		for _, row := range rows {
			items = append(items, historyEvent{
				ID:          row.ID,
				ServiceName: row.ServiceName,
				MachineID:   row.MachineID,
				Timestamp:   row.Timestamp,
				SensorType:  row.SensorType,
				SourceKind:  "valve",
				SourceID:    row.ValveNumber,
				IsOpen:      row.IsOpen,
				FlowRate:    row.FlowRate,
				AlertCount:  row.AlertCount,
				Anomalies:   splitAnomalies(row.AnomalyLabels),
			})
		}
	}

	respondWithJSON(w, http.StatusOK, struct {
		Selection querySelection `json:"selection"`
		Items     []historyEvent `json:"items"`
	}{
		Selection: selection,
		Items:     items,
	})
}

func splitAnomalies(labels string) []string {
	if labels == "" {
		return []string{}
	}
	return strings.Split(labels, ",")
}

type errorString string

func (e errorString) Error() string { return string(e) }
