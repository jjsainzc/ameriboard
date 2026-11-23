package controllers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

/**
Fuente de datos (Backend): Un script o microservicio que lea las últimas líneas de los archivos de log activos en tu servidor:

/var/log/YSFReflector/YSFReflector-YYYY-MM-DD.log (para conexiones, desconexiones y transmisiones YSF).
/var/log/mmdvm/MMDVM_Bridge-{YYYY-MM-DD}.log
o
/var/log/dvswitch/Analog_Bridge.log (para el tráfico DMR del puente hacia TGIF).

JSON, ejemplo

{
  "header": {
    "title": "Ameriboard",
    "subtitle": "Panel de monitoreo de YSF, EchoLink y DVSwitch",
    "badge": "",
    "localBridgeStatus": "—",
    "lastUpdate": "2026-11-22T09:14:00-05:00"
  },
  "stats": {
    "ysfConnectedCount": 24,
    "ysfTotalHeardToday": 68,
    "dmrActiveCount": 12,
    "dmrTotalHeardToday": 45
  },
  "ysfConnected": [
    {
      "callsign": "HC1LNM-R",
      "gateway": "74001 - Quito",
      "duration": "02:15:30",
      "status": "TX"
    },
    {
      "callsign": "KQ4HPI",
      "gateway": "Hotspot",
      "duration": "00:42:10",
      "status": "IDLE"
    }
  ],
  "ysfLastHeard": [
    {
      "time": "09:13:45",
      "callsign": "HC1LNM",
      "target": "ALL",
      "gateway": "HC1LNM-R",
      "duration": "00:18",
      "loss": "0.0%"
    }
  ],
  "dmrLastHeard": [
    {
      "time": "09:12:10",
      "callsign": "HC2GRC",
      "dmrId": "7400012",
      "talkgroup": "TG 7401",
      "source": "TGIF Network",
      "duration": "00:24"
    }
  ]
}


*/

type ReflectorConfig struct {
	Title               string
	Subtitle            string
	Badge               string
	DVSwitchTitle       string
	YSFReflector        string
	TGIFTalkgroup       string
	BridgeCall          string
	YSFLogDir           string
	DMRLogDir           string
	EchoLinkCommand     string
	EchoLinkHistoryFile string
	BridgeConfigDir     string
}

type ReflectorService struct {
	mu              sync.RWMutex
	cfg             ReflectorConfig
	echoLinkHistory *echoLinkHistoryStore
}

func NewReflectorService(cfg ReflectorConfig) *ReflectorService {
	service := &ReflectorService{cfg: cfg}
	if cfg.EchoLinkHistoryFile != "" {
		service.echoLinkHistory = newEchoLinkHistoryStore(cfg.EchoLinkHistoryFile)
	}
	return service
}

func (s *ReflectorService) SetDashboardSettings(title, subtitle, dvSwitchTitle string) {
	s.mu.Lock()
	s.cfg.Title = title
	s.cfg.Subtitle = subtitle
	s.cfg.DVSwitchTitle = dvSwitchTitle
	s.mu.Unlock()
}

// --- Estructuras JSON ---

type HeaderInfo struct {
	Title             string `json:"title"`
	Subtitle          string `json:"subtitle"`
	Badge             string `json:"badge"`
	LocalBridgeStatus string `json:"localBridgeStatus"`
	LastUpdate        string `json:"lastUpdate"`
}

type StatsInfo struct {
	YSFConnectedNow int `json:"ysfConnectedNow"`
	YSFTx24h        int `json:"ysfTx24h"`
	DMRTx24h        int `json:"dmrTx24h"`
	DVSwitchTx24h   int `json:"dvSwitchTx24h"`
	Stations24h     int `json:"stations24h"`
}

type TXEntry struct {
	Callsign  string `json:"callsign"`
	Duration  string `json:"duration"`
	Ago       string `json:"ago"`
	Mode      string `json:"mode,omitempty"`
	RxTime    string `json:"rxTime,omitempty"`
	Name      string `json:"name,omitempty"`
	Source    string `json:"source,omitempty"`
	Loss      string `json:"loss,omitempty"`
	BER       string `json:"ber,omitempty"`
	Target    string `json:"target,omitempty"`
	Gateway   string `json:"gateway,omitempty"`
	DMRID     string `json:"dmrId,omitempty"`
	Talkgroup string `json:"talkgroup,omitempty"`
	timestamp time.Time
}

type DMRActive24h struct {
	Callsign string `json:"callsign"`
	Times    int    `json:"times"`
	AirTime  string `json:"airTime"`
	LastSeen string `json:"lastSeen"`
}

type YSFStation24h struct {
	Callsign       string `json:"callsign"`
	Status         string `json:"status"`
	NewConnections int    `json:"newConnections"`
	LastConnection string `json:"lastConnection"`
}

type YSFConnectedNow struct {
	Callsign string `json:"callsign"`
	IP       string `json:"ip"`
	Type     string `json:"type"`
	Since    string `json:"since"`
}

type EchoLinkStation struct {
	Number    string `json:"number"`
	User      string `json:"user"`
	Attribute string `json:"attribute"`
}

type CombinedActivity24h struct {
	Time     string `json:"time"`
	Mode     string `json:"mode"`
	Callsign string `json:"callsign"`
	Duration string `json:"duration"`
	Detail   string `json:"detail"`
}

type DashboardStatusResponse struct {
	Header                HeaderInfo            `json:"header"`
	DVSwitchTitle         string                `json:"dvSwitchTitle"`
	Stats                 StatsInfo             `json:"stats"`
	YSFLastTX             []TXEntry             `json:"ysfLastTx"`
	DMRLastTX             []TXEntry             `json:"dmrLastTx"`
	DVSwitchLastTX        []TXEntry             `json:"dvSwitchLastTx"`
	DMRActive24h          []DMRActive24h        `json:"dmrActive24h"`
	YSFStations24h        []YSFStation24h       `json:"ysfStations24h"`
	YSFConnectedList      []YSFConnectedNow     `json:"ysfConnectedList"`
	Combined24h           []CombinedActivity24h `json:"combined24h"`
	EchoLinkConnectedList []EchoLinkStation     `json:"echolinkConnectedList"`
	EchoLinkLastHeardList []EchoLinkLastHeard   `json:"echolinkLastHeard"`
	EchoLinkUptime        string                `json:"echolinkUptime"`
	DVSwitchBridges       []DVSwitchBridge      `json:"dvSwitchBridges"`
	YSFAvailable          bool                  `json:"ysfAvailable"`
	YSFTrafficAvailable   bool                  `json:"ysfTrafficAvailable"`
	DMRAvailable          bool                  `json:"dmrAvailable"`
	EchoLinkAvailable     bool                  `json:"echolinkAvailable"`
	Warnings              []string              `json:"warnings"`
}

// --- Regex reales para YSFReflector y MMDVM_Bridge ---

var (
	// YSFReflector: snapshot de gateways/repetidores enlazados
	ysfLinkedHeaderRe = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+Currently linked repeaters/gateways:`)
	ysfLinkedEntryRe  = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Z0-9_/\-]+)\s*:\s+((\d+\.\d+\.\d+\.\d+):\d+)`)

	// YSFReflector: inicio y fin de ráfaga YSF
	ysfDataStartRe = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+Received data from\s+([A-Z0-9\-/]+)\s+to\s+([A-Z0-9\-/]+)\s+at\s+([A-Z0-9\-/]+)`)
	ysfDataEndRe   = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+Received end of transmission`)

	// MMDVM_Bridge: detección de ráfaga originada en YSF (para evitar duplicarla como DMR puro)
	mmdvmYsfNetEndRe = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+YSF,\s+received network end of transmission.*?,\s*(\d+(?:\.\d+)?)\s*seconds`)
	mmdvmYsfCallRe   = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+YSF,\s+received network data from\s+([A-Z0-9\-/]+)`)
	mmdvmYsfDataRe   = regexp.MustCompile(`YSF,\s+received network data from\s+[A-Z0-9\-/]+\s+to\s+([A-Z0-9\-/]+)\s+at\s+([A-Z0-9\-/]+)`)

	// MMDVM_Bridge: inicio y fin de transmisión en los modos de DVSwitch.
	dvSwitchBeginTxRe       = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Za-z][A-Za-z0-9-]*),\s+Begin TX:.*?src=(\d+).*?dst=([A-Z0-9\-/]+).*?metadata=([A-Z0-9\-/]+)`)
	dvSwitchTxOffRe         = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Za-z][A-Za-z0-9-]*),\s+TX state = OFF(?:,\s+DMR frame count was\s+(\d+)\s+frames)?`)
	dvSwitchNetworkHeaderRe = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Za-z][A-Za-z0-9-]*),\s+received network header from\s+([A-Z0-9-]+)(?:\s+/[A-Z]+)?\s+to\s+([A-Z0-9/]+)`)
	dvSwitchNetworkTXRe     = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Za-z][A-Za-z0-9-]*),\s+received network transmission from\s+([A-Z0-9-]+)\s+to TG\s+([A-Z0-9-]+)`)
	dvSwitchNetworkDataRe   = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Za-z][A-Za-z0-9-]*),\s+received network data from\s+([A-Z0-9-]+)\s+to\s+([A-Z0-9-]+)\s+at\s+([A-Z0-9-]+)`)
	dvSwitchNetworkEndRe    = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Za-z][A-Za-z0-9-]*),\s+received network end of transmission`)
	dvSwitchMetricModeRe    = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Za-z][A-Za-z0-9-]*),`)
	durationMetricRe        = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*seconds`)
	lossMetricRe            = regexp.MustCompile(`(\d+(?:\.\d+)?)%\s*packet loss`)
	berMetricRe             = regexp.MustCompile(`BER:\s*(\d+(?:\.\d+)?)%`)
)

type activeYSFTx struct {
	callsign string
	target   string
	gateway  string
	start    time.Time
}

type activeDVSwitchTx struct {
	mode     string
	callsign string
	srcID    string
	dstTG    string
	start    time.Time
}

type activeDVSwitchNetworkTx struct {
	mode     string
	callsign string
	target   string
	start    time.Time
}

type dmrAgg struct {
	callsign  string
	times     int
	totalSecs float64
	lastSeen  time.Time
}

func (s *ReflectorService) GetStatus(c echo.Context) error {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)

	var currentSnapshotTime time.Time
	var currentSnapshotList []YSFConnectedNow
	seenInSnapshot := make(map[string]struct{})
	inLinkedBlock := false

	stations24hMap := make(map[string]*YSFStation24h)
	dmr24hMap := make(map[string]*dmrAgg)

	var ysfTX []TXEntry
	var dmrTX []TXEntry
	var dvSwitchTX []TXEntry
	var combined []CombinedActivity24h
	var pendingYSFTx *activeYSFTx
	ysfAvailable, dmrAvailable := true, true
	warnings := make([]string, 0)

	// 1. Parsear YSFReflector
	ysfFiles, err := candidateLogFiles(cfg.YSFLogDir, "YSFReflector", now)
	if err != nil {
		ysfAvailable = false
		if !errors.Is(err, os.ErrNotExist) {
			warnings = append(warnings, "No se pueden leer los logs YSF")
			c.Logger().Errorf("No se pueden leer los logs YSF: %v", err)
		}
	} else if len(ysfFiles) == 0 {
		ysfAvailable = false
	}
	for _, path := range ysfFiles {
		f, err := os.Open(path)
		if err != nil {
			ysfAvailable = false
			warnings = append(warnings, "No se pueden leer todos los logs YSF")
			c.Logger().Errorf("No se puede abrir el log YSF: %v", err)
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")

			if m := ysfLinkedHeaderRe.FindStringSubmatch(line); len(m) == 2 {
				ts, err := parseLogTime(m[1], now.Location())
				if err == nil {
					inLinkedBlock = true
					currentSnapshotTime = ts
					currentSnapshotList = nil
					seenInSnapshot = make(map[string]struct{})
				}
				continue
			}

			if inLinkedBlock {
				if m := ysfLinkedEntryRe.FindStringSubmatch(line); len(m) == 5 {
					call := strings.TrimSpace(m[2])
					addr := strings.TrimSpace(m[3])

					key := call + "@" + addr
					if _, exists := seenInSnapshot[key]; !exists {
						seenInSnapshot[key] = struct{}{}
						currentSnapshotList = append(currentSnapshotList, YSFConnectedNow{
							Callsign: call,
							IP:       m[4],
							Type:     "Estación o gateway",
							Since:    currentSnapshotTime.Format("03:04 PM"),
						})
					}

					st := ensureYSFStation(stations24hMap, call)
					if currentSnapshotTime.After(cutoff24h) {
						st.Status = "En línea"
						if st.LastConnection == "—" {
							st.LastConnection = currentSnapshotTime.Format("03:04 PM")
						}
					}
					continue
				}
				inLinkedBlock = false
			}

			if m := ysfDataStartRe.FindStringSubmatch(line); len(m) == 5 {
				ts, err := parseLogTime(m[1], now.Location())
				if err != nil {
					continue
				}
				if pendingYSFTx != nil {
					recordYSFTx(pendingYSFTx, ts, now, cutoff24h, cfg, &ysfTX, &combined, stations24hMap)
				}
				pendingYSFTx = &activeYSFTx{
					callsign: strings.TrimSpace(m[2]),
					target:   strings.TrimSpace(m[3]),
					gateway:  strings.TrimSpace(m[4]),
					start:    ts,
				}
				continue
			}

			if m := ysfDataEndRe.FindStringSubmatch(line); len(m) == 2 {
				ts, err := parseLogTime(m[1], now.Location())
				if err != nil {
					continue
				}
				if pendingYSFTx != nil {
					recordYSFTx(pendingYSFTx, ts, now, cutoff24h, cfg, &ysfTX, &combined, stations24hMap)
					pendingYSFTx = nil
				}
				continue
			}
		}
		scanErr := scanner.Err()
		_ = f.Close()
		if scanErr != nil {
			ysfAvailable = false
			warnings = append(warnings, "No se pueden leer todos los logs YSF")
			c.Logger().Errorf("No se puede leer el log YSF: %v", scanErr)
		}
	}

	// 2. Parsear MMDVM_Bridge (soporta nombres MMDVM_Bridge-YYYY-MM-DD.log o mmdvm_bridge-YYYY-MM-DD.log)
	dmrFiles, err := candidateLogFilesAnyPrefix(cfg.DMRLogDir, []string{"MMDVM_Bridge", "mmdvm_bridge"}, now)
	if err != nil {
		dmrAvailable = false
		if !errors.Is(err, os.ErrNotExist) {
			warnings = append(warnings, "No se pueden leer los logs DVSwitch")
			c.Logger().Errorf("No se pueden leer los logs DVSwitch: %v", err)
		}
	} else if len(dmrFiles) == 0 {
		dmrAvailable = false
	}
	var lastYSFTime time.Time
	var pendingNetworkYSF *activeYSFTx
	var pendingDVSwitchTx *activeDVSwitchTx
	var pendingDVSwitchNetworkTx *activeDVSwitchNetworkTx

	for _, path := range dmrFiles {
		f, err := os.Open(path)
		if err != nil {
			dmrAvailable = false
			warnings = append(warnings, "No se pueden leer todos los logs DMR")
			c.Logger().Errorf("No se puede abrir el log DMR: %v", err)
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")

			if m := dvSwitchNetworkHeaderRe.FindStringSubmatch(line); len(m) == 5 {
				if ts, err := parseLogTime(m[1], now.Location()); err == nil {
					pendingDVSwitchNetworkTx = &activeDVSwitchNetworkTx{
						mode: m[2], callsign: m[3], target: m[4], start: ts,
					}
				}
				continue
			}
			if m := dvSwitchNetworkTXRe.FindStringSubmatch(line); len(m) == 5 {
				if ts, err := parseLogTime(m[1], now.Location()); err == nil {
					pendingDVSwitchNetworkTx = &activeDVSwitchNetworkTx{
						mode: m[2], callsign: m[3], target: "TG " + m[4], start: ts,
					}
				}
				continue
			}
			if m := dvSwitchNetworkDataRe.FindStringSubmatch(line); len(m) == 6 {
				if ts, err := parseLogTime(m[1], now.Location()); err == nil {
					pendingDVSwitchNetworkTx = &activeDVSwitchNetworkTx{
						mode: m[2], callsign: m[3], target: m[4] + " at " + m[5], start: ts,
					}
				}
			}
			if m := dvSwitchNetworkEndRe.FindStringSubmatch(line); len(m) == 3 &&
				pendingDVSwitchNetworkTx != nil && pendingDVSwitchNetworkTx.mode == m[2] {
				if ts, err := parseLogTime(m[1], now.Location()); err == nil && !ts.Before(cutoff24h) {
					duration := durationMetricRe.FindStringSubmatch(line)
					loss := lossMetricRe.FindStringSubmatch(line)
					ber := berMetricRe.FindStringSubmatch(line)
					durationValue := "—"
					if len(duration) == 2 {
						durationValue = duration[1]
					} else if elapsed := ts.Sub(pendingDVSwitchNetworkTx.start).Seconds(); elapsed > 0 {
						durationValue = fmt.Sprintf("%.1f", elapsed)
					}
					lossValue, berValue := "—", "—"
					if len(loss) == 2 {
						lossValue = loss[1] + "%"
					}
					if len(ber) == 2 {
						berValue = ber[1] + "%"
					}
					dvSwitchTX = append(dvSwitchTX, TXEntry{
						Callsign: pendingDVSwitchNetworkTx.callsign, Duration: durationValue,
						Ago: formatAgo(now.Sub(pendingDVSwitchNetworkTx.start)), Mode: pendingDVSwitchNetworkTx.mode,
						RxTime: pendingDVSwitchNetworkTx.start.Format("02/01/2006 15:04:05"),
						Name:   "—", Source: "Net", Loss: lossValue, BER: berValue,
						Target: pendingDVSwitchNetworkTx.target, timestamp: pendingDVSwitchNetworkTx.start,
					})
				}
				pendingDVSwitchNetworkTx = nil
			}

			if m := dvSwitchMetricModeRe.FindStringSubmatch(line); len(m) == 3 {
				duration := durationMetricRe.FindStringSubmatch(line)
				loss := lossMetricRe.FindStringSubmatch(line)
				ber := berMetricRe.FindStringSubmatch(line)
				if len(duration) == 2 || len(loss) == 2 || len(ber) == 2 {
					if ts, err := parseLogTime(m[1], now.Location()); err == nil {
						for i := len(dvSwitchTX) - 1; i >= 0; i-- {
							entry := &dvSwitchTX[i]
							elapsed := ts.Sub(entry.timestamp)
							if entry.Mode != m[2] || elapsed < 0 || elapsed > 30*time.Second {
								continue
							}
							if len(duration) == 2 {
								entry.Duration = duration[1]
							}
							if len(loss) == 2 {
								entry.Loss = loss[1] + "%"
							}
							if len(ber) == 2 {
								entry.BER = ber[1] + "%"
							}
							break
						}
					}
				}
			}

			// Registrar último indicativo YSF visto en el puente
			if m := mmdvmYsfCallRe.FindStringSubmatch(line); len(m) == 3 {
				if ts, err := parseLogTime(m[1], now.Location()); err == nil {
					call := strings.TrimSpace(m[2])
					if pendingNetworkYSF != nil && pendingNetworkYSF.callsign != call && !lastYSFTime.Before(cutoff24h) {
						ysfTX = append(ysfTX, TXEntry{Callsign: pendingNetworkYSF.callsign, Duration: "—", Ago: formatAgo(now.Sub(lastYSFTime)), Target: pendingNetworkYSF.target, Gateway: pendingNetworkYSF.gateway, timestamp: lastYSFTime})
					}
					if pendingNetworkYSF == nil || pendingNetworkYSF.callsign != call {
						pendingNetworkYSF = &activeYSFTx{callsign: call, start: ts}
					}
					lastYSFTime = ts
					if details := mmdvmYsfDataRe.FindStringSubmatch(line); len(details) == 3 {
						pendingNetworkYSF.target, pendingNetworkYSF.gateway = details[1], details[2]
					}
				}
				continue
			}

			// Fin de trama YSF dentro de MMDVM_Bridge
			if m := mmdvmYsfNetEndRe.FindStringSubmatch(line); len(m) == 3 {
				ts, err := parseLogTime(m[1], now.Location())
				if err != nil || ts.Before(cutoff24h) {
					continue
				}
				secs, _ := strconv.ParseFloat(m[2], 64)

				if pendingNetworkYSF == nil || ts.Before(lastYSFTime) || ts.Sub(lastYSFTime) > 30*time.Second {
					continue
				}
				call := pendingNetworkYSF.callsign
				ensureYSFStation(stations24hMap, call)
				ysfTX = append(ysfTX, TXEntry{Callsign: call, Duration: formatDurationSecs(secs), Ago: formatAgo(now.Sub(ts)), Target: pendingNetworkYSF.target, Gateway: pendingNetworkYSF.gateway, timestamp: ts})
				pendingNetworkYSF = nil
				if secs > 0.2 {
					combined = append(combined, CombinedActivity24h{
						Time:     ts.Format("15:04:05"),
						Mode:     "YSF-NET",
						Callsign: call,
						Duration: formatDurationSecs(secs),
						Detail:   fmt.Sprintf("MMDVM_Bridge (%s)", cfg.YSFReflector),
					})
				}
				continue
			}

			// Inicio de transmisión de cualquiera de las modalidades de DVSwitch.
			if m := dvSwitchBeginTxRe.FindStringSubmatch(line); len(m) == 6 {
				ts, err := parseLogTime(m[1], now.Location())
				if err != nil {
					continue
				}
				call := strings.TrimSpace(m[5])
				if call == "" || call == "N0CALL" {
					call = strings.TrimSpace(m[3])
				}
				pendingDVSwitchTx = &activeDVSwitchTx{
					mode:     m[2],
					callsign: call,
					srcID:    strings.TrimSpace(m[3]),
					dstTG:    strings.TrimSpace(m[4]),
					start:    ts,
				}
				continue
			}

			// Fin de transmisión de cualquiera de las modalidades de DVSwitch.
			if m := dvSwitchTxOffRe.FindStringSubmatch(line); len(m) == 4 {
				ts, err := parseLogTime(m[1], now.Location())
				if err != nil || ts.Before(cutoff24h) {
					pendingDVSwitchTx = nil
					continue
				}
				if pendingDVSwitchTx == nil || pendingDVSwitchTx.mode != m[2] {
					continue
				}
				frames, _ := strconv.ParseFloat(m[3], 64)
				secs := frames * 0.06
				if elapsed := ts.Sub(pendingDVSwitchTx.start).Seconds(); elapsed > 0 && elapsed < 10*60 {
					secs = elapsed
				}
				if secs <= 0 {
					secs = frames * 0.06
				}
				duration := formatDurationSecs(secs)
				target := formatDVSwitchTarget(pendingDVSwitchTx.mode, pendingDVSwitchTx.dstTG, pendingDVSwitchTx.callsign)
				dvSwitchEntry := TXEntry{
					Callsign:  pendingDVSwitchTx.callsign,
					Duration:  fmt.Sprintf("%.1f", secs),
					Ago:       formatAgo(now.Sub(ts)),
					Mode:      pendingDVSwitchTx.mode,
					RxTime:    ts.Format("02/01/2006 15:04:05"),
					Name:      "—",
					Source:    "—",
					Loss:      "—",
					BER:       "—",
					DMRID:     pendingDVSwitchTx.srcID,
					Target:    target,
					timestamp: ts,
				}
				dvSwitchTX = append(dvSwitchTX, dvSwitchEntry)
				combined = append(combined, CombinedActivity24h{
					Time:     ts.Format("15:04:05"),
					Mode:     pendingDVSwitchTx.mode,
					Callsign: pendingDVSwitchTx.callsign,
					Duration: duration,
					Detail:   target,
				})
				if pendingDVSwitchTx.mode == "DMR" {
					dmrEntry := dvSwitchEntry
					dmrEntry.Duration = duration
					dmrEntry.Talkgroup = pendingDVSwitchTx.dstTG
					dmrTX = append(dmrTX, dmrEntry)
					agg, ok := dmr24hMap[pendingDVSwitchTx.callsign]
					if !ok {
						agg = &dmrAgg{callsign: pendingDVSwitchTx.callsign}
						dmr24hMap[pendingDVSwitchTx.callsign] = agg
					}
					agg.times++
					agg.totalSecs += secs
					if ts.After(agg.lastSeen) {
						agg.lastSeen = ts
					}
				}
				pendingDVSwitchTx = nil
				continue
			}
		}
		scanErr := scanner.Err()
		_ = f.Close()
		if scanErr != nil {
			dmrAvailable = false
			warnings = append(warnings, "No se pueden leer todos los logs DMR")
			c.Logger().Errorf("No se puede leer el log DMR: %v", scanErr)
		}
	}

	if pendingNetworkYSF != nil && !lastYSFTime.Before(cutoff24h) {
		ysfTX = append(ysfTX, TXEntry{Callsign: pendingNetworkYSF.callsign, Duration: "—", Ago: formatAgo(now.Sub(lastYSFTime)), Target: pendingNetworkYSF.target, Gateway: pendingNetworkYSF.gateway, timestamp: lastYSFTime})
		ensureYSFStation(stations24hMap, pendingNetworkYSF.callsign)
	}
	if pendingYSFTx != nil && !pendingYSFTx.start.Before(cutoff24h) {
		ysfTX = append(ysfTX, TXEntry{Callsign: pendingYSFTx.callsign, Duration: "—", Ago: formatAgo(now.Sub(pendingYSFTx.start)), Target: pendingYSFTx.target, Gateway: pendingYSFTx.gateway, timestamp: pendingYSFTx.start})
		ensureYSFStation(stations24hMap, pendingYSFTx.callsign)
	}
	if pendingDVSwitchTx != nil && !pendingDVSwitchTx.start.Before(cutoff24h) {
		target := formatDVSwitchTarget(pendingDVSwitchTx.mode, pendingDVSwitchTx.dstTG, pendingDVSwitchTx.callsign)
		entry := TXEntry{
			Callsign: pendingDVSwitchTx.callsign, Duration: "—", Ago: formatAgo(now.Sub(pendingDVSwitchTx.start)),
			Mode: pendingDVSwitchTx.mode, RxTime: pendingDVSwitchTx.start.Format("02/01/2006 15:04:05"),
			Name: "—", Source: "—", Loss: "—", BER: "—", DMRID: pendingDVSwitchTx.srcID,
			Target:    target,
			timestamp: pendingDVSwitchTx.start,
		}
		dvSwitchTX = append(dvSwitchTX, entry)
		if pendingDVSwitchTx.mode == "DMR" {
			entry.Talkgroup = pendingDVSwitchTx.dstTG
			dmrTX = append(dmrTX, entry)
		}
	}
	if pendingDVSwitchNetworkTx != nil && !pendingDVSwitchNetworkTx.start.Before(cutoff24h) {
		dvSwitchTX = append(dvSwitchTX, TXEntry{
			Callsign: pendingDVSwitchNetworkTx.callsign, Duration: "—",
			Ago: formatAgo(now.Sub(pendingDVSwitchNetworkTx.start)), Mode: pendingDVSwitchNetworkTx.mode,
			RxTime: pendingDVSwitchNetworkTx.start.Format("02/01/2006 15:04:05"),
			Name:   "—", Source: "Net", Loss: "—", BER: "—",
			Target: pendingDVSwitchNetworkTx.target, timestamp: pendingDVSwitchNetworkTx.start,
		})
	}

	// Actualizar estados de estaciones YSF según último snapshot activo
	activeNowSet := make(map[string]struct{}, len(currentSnapshotList))
	for _, item := range currentSnapshotList {
		activeNowSet[item.Callsign] = struct{}{}
	}
	for call, st := range stations24hMap {
		if _, online := activeNowSet[call]; !online {
			st.Status = "Desconectado"
		}
	}

	sort.SliceStable(ysfTX, func(i, j int) bool { return ysfTX[i].timestamp.After(ysfTX[j].timestamp) })
	sort.SliceStable(dmrTX, func(i, j int) bool { return dmrTX[i].timestamp.After(dmrTX[j].timestamp) })
	sort.SliceStable(dvSwitchTX, func(i, j int) bool { return dvSwitchTX[i].timestamp.After(dvSwitchTX[j].timestamp) })
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Time > combined[j].Time
	})

	ysfStations := make([]YSFStation24h, 0, len(stations24hMap))
	uniqueCallsigns := make(map[string]struct{})
	for _, st := range stations24hMap {
		ysfStations = append(ysfStations, *st)
		uniqueCallsigns[st.Callsign] = struct{}{}
	}
	sort.Slice(ysfStations, func(i, j int) bool {
		return ysfStations[i].Callsign < ysfStations[j].Callsign
	})

	dmrActiveList := make([]DMRActive24h, 0, len(dmr24hMap))
	for _, agg := range dmr24hMap {
		uniqueCallsigns[agg.callsign] = struct{}{}
		dmrActiveList = append(dmrActiveList, DMRActive24h{
			Callsign: agg.callsign,
			Times:    agg.times,
			AirTime:  formatDurationSecs(agg.totalSecs),
			LastSeen: agg.lastSeen.Format("03:04 PM"),
		})
	}
	sort.Slice(dmrActiveList, func(i, j int) bool {
		return dmrActiveList[i].Times > dmrActiveList[j].Times
	})

	lastUpdateStr := "—"
	if !currentSnapshotTime.IsZero() {
		lastUpdateStr = currentSnapshotTime.Format("03:04 PM")
	}

	bridges, bridgeErr := readDVSwitchBridges(cfg.BridgeConfigDir)
	if bridgeErr != nil {
		c.Logger().Errorf("No se pueden leer las configuraciones de puentes DVSwitch: %v", bridgeErr)
		warnings = append(warnings, "No se pudieron leer todas las configuraciones de puentes DVSwitch.")
	}

	echoLinkConnected, echoLinkUptime, echoLinkAvailable, echoLinkErr := readEchoLinkStatus(c.Request().Context(), cfg.EchoLinkCommand)
	if echoLinkErr != nil && !errors.Is(echoLinkErr, os.ErrNotExist) && !errors.Is(echoLinkErr, exec.ErrNotFound) {
		c.Logger().Errorf("No se puede consultar EchoLink: %v", echoLinkErr)
		if !echoLinkAvailable {
			warnings = append(warnings, "No se puede consultar EchoLink")
		}
	}
	echoLinkLastHeard := make([]EchoLinkLastHeard, 0)
	if s.echoLinkHistory != nil {
		var historyErr error
		if echoLinkAvailable {
			echoLinkLastHeard, historyErr = s.echoLinkHistory.Observe(echoLinkConnected, now)
		} else {
			echoLinkLastHeard, historyErr = s.echoLinkHistory.Read()
		}
		if historyErr != nil {
			c.Logger().Errorf("No se puede actualizar el historial de EchoLink: %v", historyErr)
			warnings = append(warnings, "No se pudo actualizar el historial de EchoLink.")
		}
	}

	resp := DashboardStatusResponse{
		DVSwitchTitle: cfg.DVSwitchTitle,
		Header: HeaderInfo{
			Title: cfg.Title,
			Subtitle: strings.NewReplacer("{ysf_reflector}", cfg.YSFReflector, "{tgif_tg}", cfg.TGIFTalkgroup).Replace(cfg.Subtitle) +
				fmt.Sprintf(" · se actualiza cada 5 segundos · última lectura %s", lastUpdateStr),
			Badge:             cfg.Badge,
			LocalBridgeStatus: "—",
			LastUpdate:        lastUpdateStr,
		},
		Stats: StatsInfo{
			YSFConnectedNow: len(currentSnapshotList),
			YSFTx24h:        len(ysfTX),
			DMRTx24h:        len(dmrTX),
			DVSwitchTx24h:   len(dvSwitchTX),
			Stations24h:     len(uniqueCallsigns),
		},
		YSFLastTX:             limitUniqueTX(ysfTX, 25),
		DMRLastTX:             limitUniqueTX(dmrTX, 25),
		DVSwitchLastTX:        limitTX(dvSwitchTX, 25),
		DMRActive24h:          dmrActiveList,
		YSFStations24h:        ysfStations,
		YSFConnectedList:      currentSnapshotList,
		Combined24h:           limitCombined(combined, 50),
		EchoLinkConnectedList: echoLinkConnected,
		EchoLinkLastHeardList: echoLinkLastHeard,
		EchoLinkUptime:        echoLinkUptime,
		DVSwitchBridges:       bridges,
		YSFAvailable:          ysfAvailable,
		YSFTrafficAvailable:   ysfAvailable || dmrAvailable,
		DMRAvailable:          dmrAvailable,
		EchoLinkAvailable:     echoLinkAvailable,
		Warnings:              warnings,
	}

	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, resp)
}

func recordYSFTx(
	tx *activeYSFTx,
	end time.Time,
	now time.Time,
	cutoff24h time.Time,
	cfg ReflectorConfig,
	ysfTX *[]TXEntry,
	combined *[]CombinedActivity24h,
	stations24hMap map[string]*YSFStation24h,
) {
	if end.Before(cutoff24h) {
		return
	}
	secs := end.Sub(tx.start).Seconds()
	if secs < 0 {
		secs = 0
	}
	durStr := formatDurationSecs(secs)
	agoStr := formatAgo(now.Sub(end))

	*ysfTX = append(*ysfTX, TXEntry{
		Callsign:  tx.callsign,
		Duration:  durStr,
		Ago:       agoStr,
		Target:    tx.target,
		Gateway:   tx.gateway,
		timestamp: end,
	})
	*combined = append(*combined, CombinedActivity24h{
		Time:     tx.start.Format("15:04:05"),
		Mode:     "YSF",
		Callsign: tx.callsign,
		Duration: durStr,
		Detail:   fmt.Sprintf("Reflector %s (%s)", cfg.YSFReflector, tx.gateway),
	})
	ensureYSFStation(stations24hMap, tx.callsign)
}

func ensureYSFStation(m map[string]*YSFStation24h, call string) *YSFStation24h {
	st, ok := m[call]
	if !ok {
		st = &YSFStation24h{
			Callsign:       call,
			Status:         "Desconectado",
			LastConnection: "—",
		}
		m[call] = st
	}
	return st
}

func readEchoLinkStatus(parent context.Context, command string) ([]EchoLinkStation, string, bool, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "tlbcmd"
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, "", false, errors.New("comando EchoLink vacío")
	}

	usersArgs := append([]string{}, parts[1:]...)
	usersArgs = append(usersArgs, "users")
	usersCtx, usersCancel := context.WithTimeout(parent, time.Second)
	usersOutput, usersErr := exec.CommandContext(usersCtx, parts[0], usersArgs...).CombinedOutput()
	usersCancel()
	if usersErr != nil {
		return nil, "", false, fmt.Errorf("consultar usuarios: %w", usersErr)
	}

	uptimeArgs := append([]string{}, parts[1:]...)
	uptimeArgs = append(uptimeArgs, "uptime")
	uptimeCtx, uptimeCancel := context.WithTimeout(parent, time.Second)
	uptimeOutput, uptimeErr := exec.CommandContext(uptimeCtx, parts[0], uptimeArgs...).CombinedOutput()
	uptimeCancel()
	uptime := ""
	if uptimeErr == nil {
		uptime = strings.TrimSpace(strings.ReplaceAll(string(uptimeOutput), "Uptime", "Server Uptime"))
	} else {
		return parseEchoLinkUsers(string(usersOutput)), "", true, fmt.Errorf("consultar uptime: %w", uptimeErr)
	}

	return parseEchoLinkUsers(string(usersOutput)), uptime, true, nil
}

func parseEchoLinkUsers(output string) []EchoLinkStation {
	stations := make([]EchoLinkStation, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		number := strings.TrimSuffix(fields[0], ".")
		user := fields[1]
		if strings.EqualFold(number, "#") || strings.EqualFold(number, "Connected") ||
			strings.EqualFold(user, "User") || strings.EqualFold(user, "Nodes") {
			continue
		}
		if _, err := strconv.Atoi(number); err != nil {
			continue
		}
		attribute := ""
		if len(fields) >= 3 {
			attribute = fields[2]
		}
		key := number + ":" + user
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		stations = append(stations, EchoLinkStation{
			Number: number, User: user, Attribute: attribute,
		})
	}
	return stations
}

func candidateLogFiles(dir, prefix string, now time.Time) ([]string, error) {
	return candidateLogFilesAnyPrefix(dir, []string{prefix}, now)
}

func candidateLogFilesAnyPrefix(dir string, prefixes []string, now time.Time) ([]string, error) {
	if _, err := os.ReadDir(dir); err != nil {
		return nil, err
	}
	var files []string
	seen := make(map[string]struct{})
	for _, offset := range []int{-1, 0} {
		d := now.UTC().AddDate(0, 0, offset)
		for _, pfx := range prefixes {
			name := fmt.Sprintf("%s-%s.log", pfx, d.Format("2006-01-02"))
			full := filepath.Join(dir, name)
			if _, ok := seen[full]; ok {
				continue
			}
			if _, err := os.Stat(full); err == nil {
				seen[full] = struct{}{}
				files = append(files, full)
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}
	return files, nil
}

func parseLogTime(raw string, loc *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	ts, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.UTC)
	return ts.In(loc), err
}

func formatDurationSecs(secs float64) string {
	total := int(secs + 0.5)
	if total < 0 {
		total = 0
	}
	m := total / 60
	s := total % 60
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatDVSwitchTarget(mode, target, callsign string) string {
	switch {
	case target == "":
		return "—"
	case mode == "YSF" && target == "ALL":
		return "ALL at " + callsign
	case mode == "DMR" || mode == "NXDN" || mode == "P25" || mode == "YSF":
		return "TG " + target
	default:
		return target
	}
}

func formatAgo(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("hace %ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("hace %dm", int(d.Minutes()))
	}
	return fmt.Sprintf("hace %dh", int(d.Hours()))
}

func limitTX(s []TXEntry, max int) []TXEntry {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func limitUniqueTX(s []TXEntry, max int) []TXEntry {
	result := make([]TXEntry, 0, max)
	seen := make(map[string]struct{})
	for _, entry := range s {
		key := strings.ToUpper(strings.TrimSpace(entry.Callsign))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, entry)
		if len(result) == max {
			break
		}
	}
	return result
}

func limitCombined(s []CombinedActivity24h, max int) []CombinedActivity24h {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
