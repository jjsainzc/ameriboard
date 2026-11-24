package controllers

import (
	"bytes"
	"context"
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
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf 'tlbcmd Version 0.56\\n65536\\nerror\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{EchoLinkCommand: command})
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "ban-list"}); err == nil || !strings.Contains(err.Error(), "65536") {
		t.Fatalf("protocol error not reported: %v", err)
	}
}

func TestEchoLinkPermanentConnectRetriesInBackground(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "tlbcmd")
	counter := filepath.Join(dir, "attempts")
	script := "#!/bin/sh\necho x >> '" + counter + "'\n[ \"$(wc -l < '" + counter + "')\" -ge 3 ] && exit 0\necho 1\n"
	if err := os.WriteFile(command, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	service := NewReflectorService(ReflectorConfig{EchoLinkCommand: command})
	service.retryInterval = 20 * time.Millisecond
	t.Cleanup(service.Close)

	output, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "W1AW", Permanent: true})
	if err != nil || !strings.Contains(output, "segundo plano") {
		t.Fatalf("output=%q error=%v", output, err)
	}
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "w1aw", Permanent: true}); err == nil {
		t.Fatal("duplicate permanent connect accepted")
	}
	deadline := time.Now().Add(2 * time.Second)
	for service.hasEchoLinkRetry("W1AW") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	data, _ := os.ReadFile(counter)
	if service.hasEchoLinkRetry("W1AW") || strings.Count(string(data), "x") != 3 {
		t.Fatalf("retry did not stop after success; attempts=%d", strings.Count(string(data), "x"))
	}

	if err := os.WriteFile(counter, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "N0CALL"}); err == nil || service.hasEchoLinkRetry("N0CALL") {
		t.Fatalf("non-permanent connect should fail without retry: %v", err)
	}
	service.retryInterval = time.Hour
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "connect", Callsign: "N0CALL", Permanent: true}); err != nil {
		t.Fatal(err)
	}
	if list, _ := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "retry-list"}); !strings.HasPrefix(list, "N0CALL ") {
		t.Fatalf("retry list=%q", list)
	}
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "retry-cancel", Callsign: "n0call"}); err != nil || service.hasEchoLinkRetry("N0CALL") {
		t.Fatalf("cancel error=%v", err)
	}
	if _, err := service.ExecuteEchoLinkAction(context.Background(), EchoLinkActionRequest{Action: "retry-cancel", Callsign: "N0CALL"}); err == nil {
		t.Fatal("cancel of missing retry accepted")
	}
}
