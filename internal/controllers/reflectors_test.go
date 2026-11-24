package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestYSFReflectorInfoFromIni(t *testing.T) {
	path := filepath.Join(t.TempDir(), "YSFReflector.ini")
	if err := os.WriteFile(path, []byte("[Info]\n#Id=31645\n#Name=US EAGLES\n#Description=Florida Eagles\nId=74002\nName=EC ECUADOR\nDescription=C4FM\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	info := readYSFReflectorInfo(path)
	if info.ID != "74002" || info.Name != "EC ECUADOR" || info.Description != "C4FM" {
		t.Fatalf("unexpected YSF reflector info: %+v", info)
	}
	output := "udp 0 0 0.0.0.0:42001 0.0.0.0:* 160708/python3\n" +
		"udp 0 0 0.0.0.0:42000 0.0.0.0:* 13792/./YSFReflecto\n"
	if got := parseYSFActivePortOutput(output); got != "42000" {
		t.Fatalf("active YSF port = %q, want 42000", got)
	}
	if got := parseYSFActivePortOutput("UNCONN 0 0 0.0.0.0:42002 0.0.0.0:* users:((\"YSFReflecto\",pid=4,fd=5))"); got != "42002" {
		t.Fatalf("active YSF port from ss = %q, want 42002", got)
	}
	if got := parseYSFActivePortOutput("udp 0 0 0.0.0.0:42001 0.0.0.0:* 160708/python3\n"); got != "" {
		t.Fatalf("python port mistaken for YSF port: %q", got)
	}
}

func TestYSFReflectorConfigCaseFallback(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "YSFreflector.ini")
	newPath := filepath.Join(dir, "YSFReflector.ini")
	if err := os.WriteFile(newPath, []byte("[Info]\nName=EC ECUADOR\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openYSFReflectorConfig(oldPath, newPath)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if err := os.WriteFile(oldPath, []byte("[Info]\nName=Otro\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err = openYSFReflectorConfig(oldPath, newPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if file.Name() != oldPath {
		t.Fatalf("configured file was not preferred: %q", file.Name())
	}
}

func TestPYSFReflectorINIAndUnitPaths(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, "pysfreflector.ini")
	if err := os.WriteFile(iniPath, []byte(`[Info]
	# Id=5 digits only
	Id=74001
	# 16 characters max
	Name=EC QUITO
	# 14 characters max
	Description=YSF ROOM C4FM
	[Network]
	Port=42001
	[Log]
	FilePath=`+dir+`
	FileRoot=YSFReflector
	`), 0o600); err != nil {
		t.Fatal(err)
	}
	ini := readPYSFReflectorINI(iniPath)
	if ini.ID != "74001" || ini.Name != "EC QUITO" || ini.Description != "YSF ROOM C4FM" || ini.Port != "42001" ||
		ini.LogDir != dir || ini.LogRoot != "YSFReflector" {
		t.Fatalf("unexpected pYSF INI: %+v", ini)
	}
	unitPath := filepath.Join(dir, "ysfreflector.service")
	if err := os.WriteFile(unitPath, []byte("[Service]\nExecStart=/opt/pysfreflector/YSFReflector "+iniPath+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readPYSFReflectorUnitIni(unitPath); got != iniPath {
		t.Fatalf("pYSF INI from unit = %q, want %q", got, iniPath)
	}
	if m := pysfDataStartRe.FindStringSubmatch("M: 2024-08-16 21:16:54.636 <1077555> Received data from HC1LNM to ALL at HC1LNM-DVS"); len(m) != 5 ||
		m[2] != "HC1LNM" || m[4] != "HC1LNM-DVS" {
		t.Fatalf("pYSF traffic line was not parsed: %v", m)
	}
}

func TestYSFGatewayType(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "127.1.2.3", "localhost", "::1"} {
		if got := ysfGatewayType(ip); got != "Puente" {
			t.Fatalf("YSF gateway type for %q = %q, want Puente", ip, got)
		}
	}
	if got := ysfGatewayType("192.0.2.1"); got != "Estación o gateway" {
		t.Fatalf("remote YSF gateway type = %q", got)
	}
}

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
	script := "#!/bin/sh\nif [ \"$1\" = users ]; then printf 'tlbcmd Version 0.56\\n0\\n1. KQ4HPI-R CT\\n'; else printf 'tlbcmd Version 0.56\\n0\\nUptime: 10 days\\n'; fi\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	stations, uptime, available, err := readEchoLinkStatus(context.Background(), command)
	if err != nil || !available || len(stations) != 1 || stations[0].User != "KQ4HPI-R" || stations[0].Attribute != "CT" || uptime != "tlbcmd Version 0.56\nServer Uptime: 10 days" {
		t.Fatalf("stations=%+v uptime=%q available=%v err=%v", stations, uptime, available, err)
	}
}

func TestEchoLinkHistoryPersistsDisconnectAndDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "echolink-history.json")
	start := time.Date(2026, 11, 23, 9, 0, 0, 0, time.FixedZone("EST", -5*60*60))
	store := newEchoLinkHistoryStore(path)
	if rows, err := store.Observe([]EchoLinkStation{{User: "KQ4HPI-R"}}, start); err != nil || len(rows) != 0 {
		t.Fatalf("starting active session: rows=%+v err=%v", rows, err)
	}

	restartedStore := newEchoLinkHistoryStore(path)
	disconnectedAt := start.Add(3*time.Minute + 12*time.Second)
	rows, err := restartedStore.Observe(nil, disconnectedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Callsign != "KQ4HPI-R" ||
		!rows[0].DisconnectedAt.Equal(disconnectedAt) || rows[0].Duration != "3m 12s" {
		t.Fatalf("unexpected last-heard history: %+v", rows)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("history file permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestEchoLinkHistoryIsReturnedByStatusWhenDisconnected(t *testing.T) {
	dir := t.TempDir()
	historyPath := filepath.Join(dir, "echolink-history.json")
	start := time.Now().Add(-10 * time.Minute)
	store := newEchoLinkHistoryStore(historyPath)
	if _, err := store.Observe([]EchoLinkStation{{User: "KQ4HPI-R"}}, start); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(dir, "tlbcmd")
	script := "#!/bin/sh\nif [ \"$1\" = users ]; then printf '0\\n'; else printf 'Uptime: 1 day\\n'; fi\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	service := NewReflectorService(ReflectorConfig{
		YSFLogDir: dir, DMRLogDir: dir, EchoLinkCommand: command,
		EchoLinkHistoryFile: historyPath,
	})
	e := echo.New()
	rec := httptest.NewRecorder()
	if err := service.GetStatus(e.NewContext(httptest.NewRequest("GET", "/", nil), rec)); err != nil {
		t.Fatal(err)
	}
	var response DashboardStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.EchoLinkLastHeardList) != 1 {
		t.Fatalf("last-heard rows = %+v", response.EchoLinkLastHeardList)
	}
	entry := response.EchoLinkLastHeardList[0]
	if entry.Callsign != "KQ4HPI-R" || entry.Duration == "" || !strings.HasPrefix(entry.Duration, "10m ") {
		t.Fatalf("unexpected last-heard entry: %+v", entry)
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
	service := NewReflectorService(ReflectorConfig{DMRLogDir: dir, EchoLinkCommand: "/nonexistent/tlbcmd"})
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

func TestStatusReturnsMoreThan25ActivityRows(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	var log strings.Builder
	for i := 0; i < 30; i++ {
		start := now.Add(time.Duration(i-60) * time.Second).Format("2006-01-02 15:04:05.000")
		end := now.Add(time.Duration(i-59) * time.Second).Format("2006-01-02 15:04:05.000")
		fmt.Fprintf(&log, "M: %s YSF, received network data from CALL%02d to ALL at CALL%02d\n"+
			"M: %s YSF, received network end of transmission, 1.0 seconds\n", start, i, i, end)
	}
	if err := os.WriteFile(filepath.Join(dir, "MMDVM_Bridge-"+now.Format("2006-01-02")+".log"), []byte(log.String()), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{DMRLogDir: dir, EchoLinkCommand: "/nonexistent/tlbcmd"})
	e := echo.New()
	rec := httptest.NewRecorder()
	if err := service.GetStatus(e.NewContext(httptest.NewRequest("GET", "/", nil), rec)); err != nil {
		t.Fatal(err)
	}
	var response DashboardStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.YSFLastTX) != 30 || len(response.DVSwitchLastTX) != 30 {
		t.Fatalf("activity rows: YSF=%d DVSwitch=%d, want 30 each", len(response.YSFLastTX), len(response.DVSwitchLastTX))
	}
	if response.YSFLastTX[0].Callsign != "CALL29" || response.DVSwitchLastTX[29].Callsign != "CALL00" {
		t.Fatalf("activity rows are not ordered newest first")
	}
}

func TestDVSwitchActivityIncludesAllTransmissionModes(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	stamp := func(d time.Duration) string { return now.Add(d).Format("2006-01-02 15:04:05.000") }
	log := fmt.Sprintf(
		"I: %s DMR, Begin TX: src=7400203 rpt=740020306 dst=9 slot=2 cc=1 metadata=HC1LNM\n"+
			"M: %s DMR, TX state = OFF, DMR frame count was 46 frames\n"+
			"I: %s NXDN, Begin TX: src=7400203 rpt=740020306 dst=8465 slot=2 cc=1 metadata=HC1LNM\n"+
			"M: %s NXDN, TX state = OFF\n"+
			"M: %s NXDN, received RF end of transmission, 2.6 seconds, BER: 0.0%%\n"+
			"I: %s YSF, Begin TX: src=7400203 rpt=740020306 dst=74001 slot=2 cc=1 metadata=HC1LNM\n"+
			"M: %s YSF, TX state = OFF\n"+
			"I: %s P25, Begin TX: src=7400203 rpt=740020306 dst=740 slot=2 cc=1 metadata=HC1LNM\n"+
			"M: %s P25, TX state = OFF\n"+
			"I: %s D-Star, Begin TX: src=7400203 rpt=740020306 dst=CQCQCQ slot=2 cc=1 metadata=HC1LNM\n"+
			"M: %s D-Star, TX state = OFF\n",
		stamp(-40*time.Second), stamp(-37*time.Second), stamp(-30*time.Second), stamp(-27*time.Second),
		stamp(-26*time.Second), stamp(-20*time.Second), stamp(-18*time.Second), stamp(-10*time.Second),
		stamp(-7*time.Second), stamp(-6*time.Second), stamp(-3*time.Second),
	)
	if err := os.WriteFile(filepath.Join(dir, "MMDVM_Bridge-"+now.Format("2006-01-02")+".log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{DMRLogDir: dir, EchoLinkCommand: "/nonexistent/tlbcmd"})
	e := echo.New()
	rec := httptest.NewRecorder()
	if err := service.GetStatus(e.NewContext(httptest.NewRequest("GET", "/", nil), rec)); err != nil {
		t.Fatal(err)
	}
	var response DashboardStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	wantModes := []string{"D-Star", "P25", "YSF", "NXDN", "DMR"}
	if len(response.DVSwitchLastTX) != len(wantModes) {
		t.Fatalf("DVSwitch rows = %+v", response.DVSwitchLastTX)
	}
	if want := now.Add(-3 * time.Second).In(time.Local).Format("02/01/2006 15:04:05"); response.DVSwitchLastTX[0].RxTime != want {
		t.Fatalf("DVSwitch local timestamp = %q, want %q", response.DVSwitchLastTX[0].RxTime, want)
	}
	for i, mode := range wantModes {
		entry := response.DVSwitchLastTX[i]
		if entry.Mode != mode || entry.DMRID != "7400203" || entry.Callsign != "HC1LNM" ||
			entry.Target == "" || entry.RxTime == "" || entry.Duration == "—" {
			t.Fatalf("DVSwitch row %d = %+v, want mode %s with timestamp, target, source ID and duration", i, entry, mode)
		}
	}
	if response.DVSwitchLastTX[3].Duration != "2.6" || response.DVSwitchLastTX[3].BER != "0.0%" ||
		response.DVSwitchLastTX[0].Target != "CQCQCQ" {
		t.Fatalf("DVSwitch detail columns were not extracted: %+v", response.DVSwitchLastTX)
	}
	if len(response.DMRLastTX) != 1 || response.DMRLastTX[0].Mode != "DMR" || response.Stats.DMRTx24h != 1 ||
		response.Stats.DVSwitchTx24h != len(wantModes) {
		t.Fatalf("DMR and DVSwitch statistics are inconsistent: rows=%+v stats=%+v", response.DMRLastTX, response.Stats)
	}
}

func TestDVSwitchNetworkRowsUseBridgeLogMetrics(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	stamp := func(d time.Duration) string { return now.Add(d).Format("2006-01-02 15:04:05.000") }
	log := fmt.Sprintf(
		"M: %s D-Star, received network header from HC1LNM  /INFO to CQCQCQ\n"+
			"M: %s D-Star, received network end of transmission, 2.5 seconds, 0%% packet loss, BER: 0.0%%\n"+
			"M: %s NXDN, received network transmission from 9999 to TG 8465\n"+
			"M: %s NXDN, received network end of transmission, 4.3 seconds\n"+
			"M: %s YSF, received network data from HC1LNM-RPT to ALL at HC1LNM\n"+
			"M: %s YSF, received network end of transmission, 0.6 seconds, 0%% packet loss, BER: 0.0%%\n",
		stamp(-30*time.Second), stamp(-27*time.Second), stamp(-20*time.Second), stamp(-16*time.Second),
		stamp(-10*time.Second), stamp(-9*time.Second),
	)
	if err := os.WriteFile(filepath.Join(dir, "MMDVM_Bridge-"+now.Format("2006-01-02")+".log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{DMRLogDir: dir, EchoLinkCommand: "/nonexistent/tlbcmd"})
	e := echo.New()
	rec := httptest.NewRecorder()
	if err := service.GetStatus(e.NewContext(httptest.NewRequest("GET", "/", nil), rec)); err != nil {
		t.Fatal(err)
	}
	var response DashboardStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.DVSwitchLastTX) != 3 {
		t.Fatalf("DVSwitch network rows = %+v", response.DVSwitchLastTX)
	}
	if want := now.Add(-10 * time.Second).In(time.Local).Format("02/01/2006 15:04:05"); response.DVSwitchLastTX[0].RxTime != want {
		t.Fatalf("DVSwitch network timestamp = %q, want %q", response.DVSwitchLastTX[0].RxTime, want)
	}
	want := []struct {
		mode, callsign, target, duration, loss, ber string
	}{
		{"YSF", "HC1LNM-RPT", "ALL at HC1LNM", "0.6", "0%", "0.0%"},
		{"NXDN", "9999", "TG 8465", "4.3", "—", "—"},
		{"D-Star", "HC1LNM", "CQCQCQ", "2.5", "0%", "0.0%"},
	}
	for i, expected := range want {
		entry := response.DVSwitchLastTX[i]
		if entry.Mode != expected.mode || entry.Callsign != expected.callsign || entry.Target != expected.target ||
			entry.Duration != expected.duration || entry.Source != "Net" ||
			entry.Loss != expected.loss || entry.BER != expected.ber {
			t.Fatalf("DVSwitch row %d = %+v, want %+v", i, entry, expected)
		}
	}
	if response.Stats.DVSwitchTx24h != len(want) {
		t.Fatalf("DVSwitch 24h count = %d, want %d", response.Stats.DVSwitchTx24h, len(want))
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

func TestYSFLinkedSnapshotWithPaddedAndFullWidthCallsigns(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	oldStamp := now.Add(-2 * time.Minute).Format("2006-01-02 15:04:05.000")
	stamp := now.Format("2006-01-02 15:04:05.000")
	log := fmt.Sprintf("M: %s Currently linked repeaters/gateways:\nM: %s OLD1      : 192.0.2.1:1234 2/60\n"+
		"M: %s Currently linked repeaters/gateways:\n"+
		"M: %s WB4TWQ    : 127.0.0.1:32780 2/60\n"+
		"M: %s WB4TWQ_B  : 127.0.0.1:1437 2/60\n"+
		"M: %s WB4TWQ-DVS: 192.0.2.2:32780 0/60\n"+
		"M: %s KF0WSS/R  : 192.0.2.3:732 9/60\n"+
		"M: %s WD4KT     : 192.0.2.4:27791 1/60\n"+
		"M: %s KJ4GNU    : 192.0.2.5:4260 4/60\n"+
		"M: %s HC1LNM    : 192.0.2.6:12782 4/60\n"+
		"M: %s WB4TWQ    : 192.0.2.7:4260 1/60\n"+
		"M: %s WB4TWQ    : 192.0.2.7:45254 0/60\n",
		oldStamp, oldStamp, stamp, stamp, stamp, stamp, stamp, stamp, stamp, stamp, stamp, stamp)
	if err := os.WriteFile(filepath.Join(dir, "YSFReflector-"+now.Format("2006-01-02")+".log"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{
		YSFLogDir: dir, DMRLogDir: dir, EchoLinkCommand: "/nonexistent/tlbcmd", BridgeCall: "HC1LNM",
	})
	e := echo.New()
	rec := httptest.NewRecorder()
	if err := service.GetStatus(e.NewContext(httptest.NewRequest("GET", "/", nil), rec)); err != nil {
		t.Fatal(err)
	}
	var response DashboardStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	want := []string{"WB4TWQ", "WB4TWQ_B", "WB4TWQ-DVS", "KF0WSS/R", "WD4KT", "KJ4GNU", "HC1LNM", "WB4TWQ", "WB4TWQ"}
	if !response.YSFAvailable || response.Stats.YSFConnectedNow != len(want) || len(response.YSFConnectedList) != len(want) {
		t.Fatalf("unexpected YSF snapshot: available=%v count=%d list=%+v", response.YSFAvailable, response.Stats.YSFConnectedNow, response.YSFConnectedList)
	}
	for i, call := range want {
		if response.YSFConnectedList[i].Callsign != call {
			t.Errorf("entry %d: got %q, want %q", i, response.YSFConnectedList[i].Callsign, call)
		}
		wantType := ysfGatewayType(response.YSFConnectedList[i].IP)
		if response.YSFConnectedList[i].Type != wantType {
			t.Errorf("entry %d has inferred type %q", i, response.YSFConnectedList[i].Type)
		}
	}
	if response.Header.LocalBridgeStatus != "—" {
		t.Fatalf("local bridge status = %q, want unknown", response.Header.LocalBridgeStatus)
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
