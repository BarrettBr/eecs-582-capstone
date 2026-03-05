package api

import (
	"net/http"
	"strings"
	"time"
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

func (cfg *apiConfig) historyHandler(w http.ResponseWriter, r *http.Request) {
	selection, err := buildQuerySelection("history", r.PathValue("kind"), r.PathValue("window"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	cfg.runSelectedQuery(w, r, selection)
}

func (cfg *apiConfig) reportHandler(w http.ResponseWriter, r *http.Request) {
	selection, err := buildQuerySelection("report", r.PathValue("kind"), r.PathValue("window"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	cfg.runSelectedQuery(w, r, selection)
}

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
	Timestamp   string   `json:"timestamp"`
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
	AvgTemp          float64 `json:"avg_temp"`
	MinTemp          float64 `json:"min_temp"`
	MaxTemp          float64 `json:"max_temp"`
	AvgValveOpenRate float64 `json:"avg_valve_open_rate"`
	MinValveOpenRate float64 `json:"min_valve_open_rate"`
	MaxValveOpenRate float64 `json:"max_valve_open_rate"`
	TotalAlerts      int64   `json:"total_alerts"`
	TempAnomalies    int64   `json:"temp_anomalies"`
	ValveAnomalies   int64   `json:"valve_anomalies"`
}

func (cfg *apiConfig) runSelectedQuery(w http.ResponseWriter, r *http.Request, selection querySelection) {
	if cfg.queries == nil {
		respondWithJSON(w, http.StatusOK, struct {
			Selection querySelection `json:"selection"`
			Items     []any          `json:"items"`
		}{
			Selection: selection,
			Items:     []any{},
		})
		return
	}

	ctx := r.Context()
	sinceUnix := time.Now().UTC().Unix() - selection.Seconds

	if selection.Endpoint == "report" {
		summary, err := cfg.queries.GetReportSummarySinceUnix(ctx, sinceUnix)
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
				AvgTemp:          summary.AvgTemp,
				MinTemp:          summary.MinTemp,
				MaxTemp:          summary.MaxTemp,
				AvgValveOpenRate: summary.AvgValveOpenRate,
				MinValveOpenRate: summary.MinValveOpenRate,
				MaxValveOpenRate: summary.MaxValveOpenRate,
				TotalAlerts:      summary.TotalAlerts,
				TempAnomalies:    summary.TempAnomalies,
				ValveAnomalies:   summary.ValveAnomalies,
			},
		})
		return
	}

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
				ID:         row.ID,
				Timestamp:  row.Timestamp,
				SensorType: row.SensorType,
				SourceKind: "valve",
				SourceID:   row.ValveNumber,
				IsOpen:     row.IsOpen,
				FlowRate:   row.FlowRate,
				AlertCount: row.AlertCount,
				Anomalies:  splitAnomalies(row.AnomalyLabels),
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
