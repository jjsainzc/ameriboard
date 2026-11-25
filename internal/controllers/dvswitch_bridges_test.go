package controllers

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ameriboard/internal/monitoring"

	"github.com/labstack/echo/v4"
)

func TestReadDVSwitchBridges(t *testing.T) {
	root := t.TempDir()
	write := func(dir, contents string) {
		t.Helper()
		path := filepath.Join(root, dir)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "DVSwitch.ini"), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("MMDVM_Bridge", `
[QUANTAR]
txPort = 34103
rxPort = 34100
[P25]
txPort = 34100
rxPort = 34103
[DMR]
txPort = 31100
rxPort = 31103
`)
	write("MMDVM_Bridge-31101", `
[DMR]
txPort = 31102 ; export
rxPort = 31105
[YSF]
;txPort = 35100
;rxPort = 35103
txPort = 31105
rxPort = 31102 # import
`)
	write("MMDVM_Bridge-37040", `
[DSTAR]
txPort = 32001
rxPort = 32002
[NXDN]
txPort = 32002
rxPort = 32001
`)
	write("Quantar_Bridge", "[DMR]\ntxPort=1\nrxPort=2\n[YSF]\ntxPort=2\nrxPort=1\n")
	write("MMDVM_Bridge-split-a", "[DMR]\ntxPort=100\nrxPort=200\n")
	write("MMDVM_Bridge-split-b", "[YSF]\ntxPort=200\nrxPort=100\n")
	write("MMDVM_Bridge-duplicate", "[DMR]\ntxPort=31102\nrxPort=31105\n[YSF]\ntxPort=31105\nrxPort=31102\n")

	got, err := readDVSwitchBridges(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []DVSwitchBridge{{ModeA: "D-Star", ModeB: "NXDN"}, {ModeA: "DMR", ModeB: "YSF"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bridges = %+v, want %+v", got, want)
	}
}

func TestParseDVSwitchBridgesRequiresCompleteReversedPorts(t *testing.T) {
	got, err := parseDVSwitchBridges(strings.NewReader(`
[DMR]
txPort=1
rxPort=2
[YSF]
;txPort=2
rxPort=1
[NXDN]
txPort=1
rxPort=2
[P25]
txPort=4
rxPort=4
[DSTAR]
txPort=4
rxPort=4
`))
	if err != nil || len(got) != 0 {
		t.Fatalf("unexpected bridges = %+v, error = %v", got, err)
	}
	if _, err := parseDVSwitchBridges(strings.NewReader("[DMR]\ntxPort=invalid\n")); err == nil {
		t.Fatal("invalid port must be reported")
	}
}

func TestReadDVSwitchBridgesReportsInvalidFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "MMDVM_Bridge-test")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "DVSwitch.ini"), []byte("[DMR]\ntxPort=bad\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDVSwitchBridges(root); err == nil {
		t.Fatal("invalid configuration must be reported")
	}
}

func TestReflectorStatusIncludesConfiguredBridgesWithoutLogs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "MMDVM_Bridge-1")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	config := "[DMR]\ntxPort=31102\nrxPort=31105\n[YSF]\ntxPort=31105\nrxPort=31102\n"
	if err := os.WriteFile(filepath.Join(dir, "DVSwitch.ini"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	disabled := monitoring.Options{}
	service := NewReflectorService(ReflectorConfig{
		BridgeConfigDir: root, YSFLogDir: root, DMRLogDir: root, EchoLinkCommand: "/nonexistent/tlbcmd",
		Monitoring: &disabled,
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
	want := []DVSwitchBridge{{ModeA: "DMR", ModeB: "YSF"}}
	if !reflect.DeepEqual(response.DVSwitchBridges, want) {
		t.Fatalf("status bridges = %+v, want %+v", response.DVSwitchBridges, want)
	}
}
