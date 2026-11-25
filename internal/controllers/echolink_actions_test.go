package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEchoLinkActionDiagnostics(t *testing.T) {
	command := filepath.Join(t.TempDir(), "tlbcmd")
	script := "#!/bin/sh\nprintf 'bridge error: station unavailable\\n' >&2\nexit 7\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	service := NewReflectorService(ReflectorConfig{EchoLinkCommand: command})

	_, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "mute", Callsign: "TEST1"})
	if err == nil || !strings.Contains(logs.String(), "exit status 7") ||
		!strings.Contains(logs.String(), "bridge error: station unavailable") {
		t.Fatalf("error=%v logs=%q", err, logs.String())
	}

	logs.Reset()
	script = "#!/bin/sh\nprintf '%s\\n' \"$*\" >&2\nexit 7\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	_, err = service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "message", Message: "texto privado"})
	if err == nil || strings.Contains(logs.String(), "texto privado") || !strings.Contains(logs.String(), "exit status 7") {
		t.Fatalf("message error=%v logs=%q", err, logs.String())
	}
}

func TestEchoLinkDiagnosticOutputRedactionAndLimit(t *testing.T) {
	request := EchoLinkActionRequest{IP: "192.0.2.1", Message: "privado"}
	result := echoLinkDiagnosticOutput("ban", "Error 192.0.2.1 privado "+strings.Repeat("x", 3000), nil, request)
	if strings.Contains(result, request.IP) || strings.Contains(result, request.Message) ||
		!strings.HasSuffix(result, "[truncada]") || len(result) > 2060 {
		t.Fatalf("unexpected diagnostic: %q", result)
	}
	result = echoLinkDiagnosticOutput("mute", "contraseña", []string{"--password=contraseña"}, request)
	if strings.Contains(result, "contraseña") || !strings.Contains(result, "omitida") {
		t.Fatalf("configured command output not withheld: %q", result)
	}
}

func TestEchoLinkBanCommandsAndInputValidation(t *testing.T) {
	command := filepath.Join(t.TempDir(), "tlbcmd")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}

	service := NewReflectorService(ReflectorConfig{EchoLinkCommand: command})
	for _, test := range []struct {
		request EchoLinkActionRequest
		want    string
	}{
		{EchoLinkActionRequest{Action: "ban-list"}, "ban\nlist"},
		{EchoLinkActionRequest{Action: "unban", Callsign: "W1AW"}, "ban\ndelete\nW1AW"},
	} {
		output, err := service.ExecuteEchoLinkAction(context.Background(), test.request)
		if err != nil || output != test.want {
			t.Errorf("action %q: output=%q error=%v", test.request.Action, output, err)
		}
	}
	for _, request := range []EchoLinkActionRequest{
		{Action: "unban", Callsign: "W1AW;shutdown"},
		{Action: "disconnect", Callsign: "ALL"},
		{Action: "ban", Callsign: "W1AW", IP: "host;shutdown"},
		{Action: "message", Message: "Hola;shutdown"},
		{Action: "message", Message: "Hola\nshutdown"},
	} {
		if _, err := service.ExecuteEchoLinkAction(context.Background(), request); err == nil {
			t.Errorf("accepted unsafe request: %+v", request)
		}
	}
}

func TestEchoLinkActionRejectsNonzeroProtocolCode(t *testing.T) {
	command := filepath.Join(t.TempDir(), "tlbcmd")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf 'tlbcmd Version 0.56\\n200003\\nStation not found.\\n'\nexit 67\n"), 0700); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{EchoLinkCommand: command})
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "ban-list"}); err == nil || !strings.Contains(err.Error(), "200003") {
		t.Fatalf("protocol error not reported: %v", err)
	}
}

func TestEchoLinkPermanentReconnectAndRestore(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "tlbcmd")
	counter := filepath.Join(dir, "attempts")
	available := filepath.Join(dir, "available")
	connected := filepath.Join(dir, "connected")
	state := filepath.Join(dir, "permanent.json")
	script := "#!/bin/sh\ncase \"$1\" in\n" +
		"users) printf '0\\n'; [ ! -f '" + connected + "' ] || printf '1. W1AW P\\n'; exit 0;;\n" +
		"connect) echo x >> '" + counter + "'; if [ -f '" + available + "' ]; then touch '" + connected + "'; printf '0\\nConnecting to \"W1AW\".\\n'; exit 0; fi; printf '200003\\nStation not found.\\n'; exit 67;;\n" +
		"disconnect) rm -f '" + connected + "'; printf '0\\n'; exit 0;;\nesac\nexit 1\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	cfg := ReflectorConfig{EchoLinkCommand: command, EchoLinkPermanentFile: state}
	service := NewReflectorService(cfg)
	service.retryInterval = 20 * time.Millisecond
	output, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "W1AW", Permanent: true})
	if err != nil || !strings.Contains(output, "segundo plano") {
		t.Fatalf("output=%q error=%v", output, err)
	}
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "w1aw", Permanent: true}); err == nil {
		t.Fatal("duplicate permanent connect accepted")
	}
	failed := waitForPermanent(t, service, func(entry echoLinkPermanent) bool { return entry.State == "error" })
	if failed.LastAttempt == nil || !strings.Contains(failed.LastResult, "200003") {
		t.Fatalf("failed attempt not recorded: %+v", failed)
	}
	if err := os.WriteFile(available, nil, 0600); err != nil {
		t.Fatal(err)
	}
	first := waitForPermanent(t, service, func(entry echoLinkPermanent) bool { return entry.State == "conectado" })
	if first.LastConnected == nil || first.LastAttempt == nil {
		t.Fatalf("connection not recorded: %+v", first)
	}
	if err := os.Remove(connected); err != nil {
		t.Fatal(err)
	}
	waitForPermanent(t, service, func(entry echoLinkPermanent) bool {
		return entry.State == "conectado" && entry.Attempts > first.Attempts
	})
	service.Close()
	if err := os.Remove(connected); err != nil {
		t.Fatal(err)
	}
	service = NewReflectorService(cfg)
	service.retryInterval = 20 * time.Millisecond
	t.Cleanup(service.Close)
	if err := service.RestoreEchoLinkConnections(); err != nil {
		t.Fatal(err)
	}
	waitForPermanent(t, service, func(entry echoLinkPermanent) bool {
		return entry.State == "conectado" && entry.Attempts > first.Attempts+1
	})
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "retry-cancel", Callsign: "w1aw"}); err != nil || service.hasEchoLinkRetry("W1AW") {
		t.Fatalf("cancel error=%v", err)
	}
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "retry-cancel", Callsign: "W1AW"}); err == nil {
		t.Fatal("cancel of missing retry accepted")
	}
	data, err := os.ReadFile(state)
	if err != nil || string(data) != "[]" {
		t.Fatalf("removed entry persisted: %q, %v", data, err)
	}
	if _, err := os.Stat(connected); !os.IsNotExist(err) {
		t.Fatalf("station remains connected: %v", err)
	}
}

func waitForPermanent(t *testing.T, service *ReflectorService, ready func(echoLinkPermanent) bool) echoLinkPermanent {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		output, err := service.echoLinkPermanentList()
		if err != nil {
			t.Fatal(err)
		}
		var entries []echoLinkPermanent
		if err := json.Unmarshal([]byte(output), &entries); err != nil {
			t.Fatal(err)
		}
		if len(entries) == 1 && ready(entries[0]) {
			return entries[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	output, _ := service.echoLinkPermanentList()
	t.Fatalf("permanent connection did not reach expected state: %s", output)
	return echoLinkPermanent{}
}

func TestEchoLinkPermanentRejectsUnwritableState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{EchoLinkPermanentFile: filepath.Join(path, "state.json")})
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "W1AW", Permanent: true}); err == nil || service.hasEchoLinkRetry("W1AW") {
		t.Fatalf("unwritable state accepted: %v", err)
	}
}

func TestEchoLinkPermanentRecognizesResolvedAlreadyConnectedTarget(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "tlbcmd")
	registered := filepath.Join(dir, "registered")
	disconnected := filepath.Join(dir, "disconnected")
	script := "#!/bin/sh\ncase \"$1\" in\n" +
		"users) printf '0\\n'; [ ! -f '" + registered + "' ] || printf '1. W1AW P\\n'; exit 0;;\n" +
		"connect) touch '" + registered + "'; printf '200006\\nAlready connected to \"W1AW\".\\n'; exit 70;;\n" +
		"disconnect) printf '%s' \"$2\" > '" + disconnected + "'; printf '0\\n'; exit 0;;\nesac\nexit 1\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{EchoLinkCommand: command})
	service.retryInterval = 20 * time.Millisecond
	t.Cleanup(service.Close)
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "123456", Permanent: true}); err != nil {
		t.Fatal(err)
	}
	entry := waitForPermanent(t, service, func(entry echoLinkPermanent) bool { return entry.State == "conectado" })
	if entry.Resolved != "W1AW" || entry.Attempts != 1 {
		t.Fatalf("unexpected resolved entry: %+v", entry)
	}
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "retry-cancel", Callsign: "123456"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(disconnected)
	if err != nil || string(data) != "W1AW" {
		t.Fatalf("disconnected target=%q error=%v", data, err)
	}
}

func TestEchoLinkPermanentWaitsForInactiveTLBConnection(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "tlbcmd")
	counter := filepath.Join(dir, "attempts")
	script := "#!/bin/sh\ncase \"$1\" in\n" +
		"users) printf '0\\n1. W1AW IxP\\n'; exit 0;;\n" +
		"connect) echo x >> '" + counter + "'; exit 0;;\n" +
		"disconnect) printf '0\\n'; exit 0;;\nesac\nexit 1\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{EchoLinkCommand: command})
	service.retryInterval = 20 * time.Millisecond
	t.Cleanup(service.Close)
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "W1AW", Permanent: true}); err != nil {
		t.Fatal(err)
	}
	entry := waitForPermanent(t, service, func(entry echoLinkPermanent) bool { return entry.State == "esperando" })
	if entry.Attempts != 0 || entry.LastConnected != nil {
		t.Fatalf("inactive connection reported as connected: %+v", entry)
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := os.Stat(counter); !os.IsNotExist(err) {
		t.Fatalf("sent unnecessary duplicate connect: %v", err)
	}
}
