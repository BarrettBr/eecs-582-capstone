package config

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

func getUint16Env(key string, fallback uint16) (uint16, error) {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		if n < 0 || n > 65535 {
			return 0, fmt.Errorf("%s must be between 0 and 65535", key)
		}
		return uint16(n), nil
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
