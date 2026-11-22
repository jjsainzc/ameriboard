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

func TestParseEchoLinkUsers(t *testing.T) {
	output := "# User Attribute\n1 W2LRC-R T\n2 12345 R\n1 W2LRC-R T\n"
	got := parseEchoLinkUsers(output)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique stations, got %d", len(got))
	}
	if got[0].User != "W2LRC-R" || got[0].Attribute != "T" {
		t.Fatalf("unexpected first station: %+v", got[0])
	}
	if got[1].Number != "2" || got[1].User != "12345" || got[1].Attribute != "R" {
		t.Fatalf("unexpected second station: %+v", got[1])
	}
}

func TestLimitUniqueTXKeepsLatestPerCallsign(t *testing.T) {
	got := limitUniqueTX([]TXEntry{
		{Callsign: "HC1AAA", Ago: "hace 1m"},
		{Callsign: "HC1BBB", Ago: "hace 2m"},
		{Callsign: "HC1AAA", Ago: "hace 3m"},
	}, 25)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique entries, got %d", len(got))
	}
	if got[0].Callsign != "HC1AAA" || got[1].Callsign != "HC1BBB" {
		t.Fatalf("unexpected entries: %+v", got)
	}
}
