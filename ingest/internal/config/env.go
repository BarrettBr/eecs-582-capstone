package config

/*
Name: ingest/internal/config/env.go
Description: Reads primitive config values from environment variables with simple fallback helpers.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-13
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer helper comments for config loading.
Preconditions:
- Environment variables are available through the current process.
Acceptable Input Values/Types:
- String, integer, duration, and bool env var values.
Unacceptable Input Values/Types:
- Malformed values that do not match the requested type.
Postconditions:
- Returns parsed config helper values or parse errors.
Return Values/Types:
- Helper functions return typed values and optional errors.
Error/Exception Conditions:
- Parse failures return descriptive errors.
Side Effects:
- Reads process environment values.
Invariants:
- Empty env values always fall back to the provided default.
Known Faults:
- These helpers do not support complex list or map types.
*/

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return d, nil
	}
	return fallback, nil
}

func getPositiveDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	d, err := getDurationEnv(key, fallback)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be > 0", key)
	}
	return d, nil
}

func getIntEnv(key string, fallback int) (int, error) {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return n, nil
	}
	return fallback, nil
}

func getPositiveIntEnv(key string, fallback int) (int, error) {
	n, err := getIntEnv(key, fallback)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be > 0", key)
	}
	return n, nil
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("parse %s: %w", key, err)
		}
		return b, nil
	}
	return fallback, nil
}
