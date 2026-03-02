package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type historyCursor struct {
	Timestamp string `json:"timestamp"`
	ID        int64  `json:"id"`
}

func encodeCursor(cursor historyCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode cursor json: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(raw string) (historyCursor, error) {
	if raw == "" {
		return historyCursor{}, fmt.Errorf("cursor is empty")
	}

	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return historyCursor{}, fmt.Errorf("decode cursor base64: %w", err)
	}

	var cursor historyCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return historyCursor{}, fmt.Errorf("decode cursor json: %w", err)
	}
	if cursor.Timestamp == "" || cursor.ID <= 0 {
		return historyCursor{}, fmt.Errorf("cursor missing required fields")
	}

	return cursor, nil
}
