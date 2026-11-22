package controllers

import (
	"testing"
	"time"
)

func TestParseLogTimeFractionalSeconds(t *testing.T) {
	for _, value := range []string{
		"2026-11-22 12:34:56",
		"2026-11-22 12:34:56.1",
		"2026-11-22 12:34:56.12",
		"2026-11-22 12:34:56.123",
	} {
		if _, err := parseLogTime(value, time.UTC); err != nil {
			t.Errorf("parseLogTime(%q): %v", value, err)
		}
	}
}
