package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
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
	output := "tlbcmd Version 0.56 compiled May 27 2024 21:39:12\n0\n1. KQ4HPI-R CT\n2 12345 R\n1. KQ4HPI-R CT\n"
	got := parseEchoLinkUsers(output)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique stations, got %d", len(got))
	}
	if got[0].Number != "1" || got[0].User != "KQ4HPI-R" || got[0].Attribute != "CT" {
		t.Fatalf("unexpected first station: %+v", got[0])
	}
	if got[1].Number != "2" || got[1].User != "12345" || got[1].Attribute != "R" {
		t.Fatalf("unexpected second station: %+v", got[1])
	}
}

func TestEchoLinkCommandOutput(t *testing.T) {
	command := filepath.Join(t.TempDir(), "tlbcmd")
	script := "#!/bin/sh\nif [ \"$1\" = users ]; then printf 'tlbcmd Version 0.56\\n0\\n1. KQ4HPI-R CT\\n'; else printf 'Uptime: 1 day\\n'; fi\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	stations, uptime, available, err := readEchoLinkStatus(context.Background(), command)
	if err != nil || !available || len(stations) != 1 || stations[0].User != "KQ4HPI-R" || stations[0].Attribute != "CT" || uptime != "Server Uptime: 1 day" {
		t.Fatalf("stations=%+v uptime=%q available=%v err=%v", stations, uptime, available, err)
	}
}

func TestBridgeTrafficAndLatestFirst(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	stamp := func(d time.Duration) string { return now.Add(d).Format("2006-01-02 15:04:05.000") }
	log := fmt.Sprintf("I: %s DMR, Begin TX: src=100 rpt=0 dst=42 slot=2 cc=0 metadata=OLD1\n"+
		"M: %s DMR, TX state = OFF, DMR frame count was 20 frames\n"+
		"M: %s YSF, received network data from     OLD1 to        ALL at OLD1\n"+
		"M: %s YSF, received network end of transmission (expired), 0.1 seconds, 0%% packet loss, BER: 0.0%%\n"+
		"M: %s YSF, received network data from     KQ4HPI to        ALL at KQ4HPI\n"+
		"I: %s DMR, Begin TX: src=3201418 rpt=0 dst=37040 slot=2 cc=0 metadata=KQ4HPI\n"+
		"M: %s DMR, TX state = OFF, DMR frame count was 100 frames\n"+
		"M: %s YSF, received network end of transmission (expired), 0.2 seconds, 0%% packet loss, BER: 0.0%%\n",
		stamp(-25*time.Second), stamp(-23*time.Second), stamp(-20*time.Second), stamp(-19*time.Second), stamp(-10*time.Second),
		stamp(-10*time.Second), stamp(-5*time.Second), stamp(-5*time.Second))
	if err := os.WriteFile(filepath.Join(dir, "MMDVM_Bridge-"+now.Format("2006-01-02")+".log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{YSFLogDir: dir, DMRLogDir: dir, EchoLinkCommand: "/nonexistent/tlbcmd"})
	e := echo.New()
	rec := httptest.NewRecorder()
	if err := service.GetStatus(e.NewContext(httptest.NewRequest("GET", "/", nil), rec)); err != nil {
		t.Fatal(err)
	}
	var response DashboardStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.YSFLastTX) != 2 || response.YSFLastTX[0].Callsign != "KQ4HPI" || response.YSFLastTX[1].Callsign != "OLD1" {
		t.Fatalf("YSF: %+v", response.YSFLastTX)
	}
	if len(response.DMRLastTX) != 2 || response.DMRLastTX[0].Callsign != "KQ4HPI" || response.DMRLastTX[1].Callsign != "OLD1" {
		t.Fatalf("DMR: %+v", response.DMRLastTX)
	}
	if response.DMRLastTX[0].DMRID != "3201418" || response.DMRLastTX[0].Talkgroup != "37040" ||
		response.YSFLastTX[0].Target != "ALL" || response.YSFLastTX[0].Gateway != "KQ4HPI" || !response.YSFTrafficAvailable {
		t.Fatalf("missing traffic details: %+v %+v", response.DMRLastTX, response.YSFLastTX)
	}
	log += fmt.Sprintf("M: %s YSF, received network data from NEW1 to ALL at NEW1\nI: %s DMR, Begin TX: src=123 rpt=0 dst=42 slot=2 cc=0 metadata=NEW1\n",
		stamp(-2*time.Second), stamp(-2*time.Second))
	if err := os.WriteFile(filepath.Join(dir, "MMDVM_Bridge-"+now.Format("2006-01-02")+".log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	if err := service.GetStatus(e.NewContext(httptest.NewRequest("GET", "/", nil), rec)); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.YSFLastTX[0].Callsign != "NEW1" || response.DMRLastTX[0].Callsign != "NEW1" {
		t.Fatalf("active traffic should be first: YSF=%+v DMR=%+v", response.YSFLastTX, response.DMRLastTX)
	}
}

func TestLogTimesAndFilesUseUTC(t *testing.T) {
	local := time.FixedZone("local", -5*60*60)
	ts, err := parseLogTime("2026-11-23 01:00:00.123", local)
	if err != nil || ts.Hour() != 20 || ts.Day() != 22 {
		t.Fatalf("unexpected local time: %v, %v", ts, err)
	}
	dir := t.TempDir()
	name := filepath.Join(dir, "MMDVM_Bridge-2026-11-23.log")
	if err := os.WriteFile(name, nil, 0600); err != nil {
		t.Fatal(err)
	}
	files, err := candidateLogFiles(dir, "MMDVM_Bridge", time.Date(2026, 11, 22, 20, 0, 0, 0, local))
	if err != nil || len(files) != 1 || files[0] != name {
		t.Fatalf("files = %v, error = %v", files, err)
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
