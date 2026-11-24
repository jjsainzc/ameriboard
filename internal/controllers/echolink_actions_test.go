package controllers

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
