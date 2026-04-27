package api

import (
	"net/http"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

type machineHistoryResponse struct {
	ServiceName string    `json:"service_name"`
	MachineID   string    `json:"machine_id"`
	EventType   string    `json:"event_type"`
	Window      string    `json:"window"`
	Labels      []string  `json:"labels"`
	Values      []float64 `json:"values"`
	AlertCounts []int64   `json:"alert_counts"`
}

// description: Returns the last hour chart series for one machine under one service.
// input: machineId path value plus service_name and optional window query params.
// output: Writes chart-friendly time series payload as JSON.
func (cfg *apiConfig) machineHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.queries == nil || cfg.status == nil {
		respondWithError(w, http.StatusServiceUnavailable, "machine history unavailable", nil)
		return
	}

	machineID := r.PathValue("machineId")
	serviceName := r.URL.Query().Get("service_name")
	window := r.URL.Query().Get("window")
	if machineID == "" || serviceName == "" {
		respondWithError(w, http.StatusBadRequest, "machineId path and service_name query are required", nil)
		return
	}
	if window == "" {
		window = "hour"
	}
	if window != "hour" {
		respondWithError(w, http.StatusBadRequest, "only window=hour is supported", nil)
		return
	}

	var eventType string
	snapshot := cfg.status.StatusSnapshot()
	for _, service := range snapshot.Services {
		if service.Name != serviceName {
			continue
		}
		eventType = service.EventType
		foundMachine := false
		for _, machine := range service.Machines {
			if machine.ID == machineID {
				foundMachine = true
				break
			}
		}
		if !foundMachine {
			respondWithError(w, http.StatusNotFound, "machine not found for service", nil)
			return
		}
		break
	}
	if eventType == "" {
		respondWithError(w, http.StatusNotFound, "service not found", nil)
		return
	}

	sinceUnix := time.Now().UTC().Add(-1 * time.Hour).Unix()
	response := machineHistoryResponse{
		ServiceName: serviceName,
		MachineID:   machineID,
		EventType:   eventType,
		Window:      "hour",
		Labels:      []string{},
		Values:      []float64{},
		AlertCounts: []int64{},
	}

	switch eventType {
	case "temperature":
		rows, err := cfg.queries.GetTempMachineHistorySinceUnix(r.Context(), database.GetTempMachineHistorySinceUnixParams{
			ServiceName: serviceName,
			MachineID:   machineID,
			SinceUnix:   sinceUnix,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed machine history query", err)
			return
		}
		for _, row := range rows {
			response.Labels = append(response.Labels, time.Unix(row.Timestamp, 0).UTC().Format("15:04:05"))
			response.Values = append(response.Values, row.AvgValue)
			response.AlertCounts = append(response.AlertCounts, row.AlertCount)
		}
	case "valve":
		rows, err := cfg.queries.GetValveMachineHistorySinceUnix(r.Context(), database.GetValveMachineHistorySinceUnixParams{
			ServiceName: serviceName,
			MachineID:   machineID,
			SinceUnix:   sinceUnix,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed machine history query", err)
			return
		}
		for _, row := range rows {
			response.Labels = append(response.Labels, time.Unix(row.Timestamp, 0).UTC().Format("15:04:05"))
			response.Values = append(response.Values, row.AvgValue)
			response.AlertCounts = append(response.AlertCounts, row.AlertCount)
		}
	default:
		respondWithError(w, http.StatusBadRequest, "unsupported service event type", nil)
		return
	}

	respondWithJSON(w, http.StatusOK, response)
}
