package controllers

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
    "title": "CONFERENCIA AMERINET",
    "subtitle": "Reflector YSF #74001 • Puente TGIF DMR TG 7401",
    "badge": "Reflector + TGIF DMR",
    "localBridgeStatus": "Activo • Enlazado",
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
	ListenAddr    string
	Title         string
	YSFReflector  string
	TGIFTalkgroup string
	BridgeCall    string
	YSFLogDir     string
	DMRLogDir     string
	CORSOrigins   []string
}

type ReflectorService struct {
	mu  sync.RWMutex
	cfg ReflectorConfig
}

func NewReflectorService(cfg ReflectorConfig) *ReflectorService {
	return &ReflectorService{cfg: cfg}
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
	Stations24h     int `json:"stations24h"`
}

type TXEntry struct {
	Callsign string `json:"callsign"`
	Duration string `json:"duration"`
	Ago      string `json:"ago"`
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
	Type     string `json:"type"`
	Since    string `json:"since"`
}

type CombinedActivity24h struct {
	Time     string `json:"time"`
	Mode     string `json:"mode"`
	Callsign string `json:"callsign"`
	Duration string `json:"duration"`
	Detail   string `json:"detail"`
}

type DashboardStatusResponse struct {
	Header           HeaderInfo            `json:"header"`
	Stats            StatsInfo             `json:"stats"`
	YSFLastTX        []TXEntry             `json:"ysfLastTx"`
	DMRLastTX        []TXEntry             `json:"dmrLastTx"`
	DMRActive24h     []DMRActive24h        `json:"dmrActive24h"`
	YSFStations24h   []YSFStation24h       `json:"ysfStations24h"`
	YSFConnectedList []YSFConnectedNow     `json:"ysfConnectedList"`
	Combined24h      []CombinedActivity24h `json:"combined24h"`
}

// --- Regex reales para YSFReflector y MMDVM_Bridge ---

var (
	// YSFReflector: snapshot de gateways/repetidores enlazados
	ysfLinkedHeaderRe = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+Currently linked repeaters/gateways:`)
	ysfLinkedEntryRe  = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([A-Z0-9\-/]+)\s+:\s+(\d+\.\d+\.\d+\.\d+:\d+)`)

	// YSFReflector: inicio y fin de ráfaga YSF
	ysfDataStartRe = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+Received data from\s+([A-Z0-9\-/]+)\s+to\s+([A-Z0-9\-/]+)\s+at\s+([A-Z0-9\-/]+)`)
	ysfDataEndRe   = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+Received end of transmission`)

	// MMDVM_Bridge: detección de ráfaga originada en YSF (para evitar duplicarla como DMR puro)
	mmdvmYsfNetEndRe = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+YSF,\s+received network end of transmission.*?,\s*(\d+(?:\.\d+)?)\s*seconds`)
	mmdvmYsfCallRe   = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+YSF,\s+(?:received network data from|GPS Position from)\s+([A-Z0-9\-/]+)`)

	// MMDVM_Bridge: inicio y fin de transmisión DMR
	dmrBeginTxRe = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+DMR,\s+Begin TX:.*?src=(\d+).*?dst=(\d+).*?metadata=([A-Z0-9\-/]+)`)
	dmrTxOffRe   = regexp.MustCompile(`^[MI]:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+DMR,\s+TX state = OFF,\s+DMR frame count was\s+(\d+)\s+frames`)
)

type activeYSFTx struct {
	callsign string
	target   string
	gateway  string
	start    time.Time
}

type activeDMRTx struct {
	callsign string
	srcID    string
	dstTG    string
	start    time.Time
	fromYSF  bool
}

type dmrAgg struct {
	callsign  string
	times     int
	totalSecs float64
	lastSeen  time.Time
}

func main() {
	cfg := ReflectorConfig{
		ListenAddr:    getEnv("LISTEN_ADDR", ":8080"),
		Title:         getEnv("DASHBOARD_TITLE", "CONFERENCIA AMERINET"),
		YSFReflector:  getEnv("YSF_REFLECTOR_ID", "74001"),
		TGIFTalkgroup: getEnv("TGIF_TG", "7401"),
		BridgeCall:    getEnv("BRIDGE_CALLSIGN", "HC1LNM"),
		YSFLogDir:     getEnv("YSF_LOG_DIR", "/var/log/YSFReflector"),
		DMRLogDir:     getEnv("DMR_LOG_DIR", "/var/log/mmdvm"),
		CORSOrigins:   splitCSV(getEnv("CORS_ORIGINS", "http://localhost:4200")),
	}

	svc := NewReflectorService(cfg)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.CORSOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodOptions},
	}))

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	api := e.Group("/api")
	api.GET("/reflector-status", svc.GetStatus)

	log.Printf("Iniciando servidor en %s | YSF=%s | TGIF=%s", cfg.ListenAddr, cfg.YSFReflector, cfg.TGIFTalkgroup)
	if err := e.Start(cfg.ListenAddr); err != nil {
		log.Fatal(err)
	}
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
	var combined []CombinedActivity24h
	var pendingYSFTx *activeYSFTx

	// 1. Parsear YSFReflector
	ysfFiles := candidateLogFiles(cfg.YSFLogDir, "YSFReflector", now)
	for _, path := range ysfFiles {
		f, err := os.Open(path)
		if err != nil {
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
				if m := ysfLinkedEntryRe.FindStringSubmatch(line); len(m) == 4 {
					call := strings.TrimSpace(m[2])
					addr := strings.TrimSpace(m[3])

					connType := "Estación o gateway"
					if strings.HasPrefix(addr, "127.0.0.1:") || (cfg.BridgeCall != "" && strings.EqualFold(call, cfg.BridgeCall)) {
						connType = "Puente local"
					}

					key := call + "@" + addr
					if _, exists := seenInSnapshot[key]; !exists {
						seenInSnapshot[key] = struct{}{}
						currentSnapshotList = append(currentSnapshotList, YSFConnectedNow{
							Callsign: call,
							Type:     connType,
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
		_ = f.Close()
	}

	// 2. Parsear MMDVM_Bridge (soporta nombres MMDVM_Bridge-YYYY-MM-DD.log o mmdvm_bridge-YYYY-MM-DD.log)
	dmrFiles := candidateLogFilesAnyPrefix(cfg.DMRLogDir, []string{"MMDVM_Bridge", "mmdvm_bridge"}, now)
	var lastYSFCall string
	var lastYSFTime time.Time
	var pendingDMRTx *activeDMRTx

	for _, path := range dmrFiles {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")

			// Registrar último indicativo YSF visto en el puente
			if m := mmdvmYsfCallRe.FindStringSubmatch(line); len(m) == 3 {
				if ts, err := parseLogTime(m[1], now.Location()); err == nil {
					lastYSFTime = ts
					lastYSFCall = strings.TrimSpace(m[2])
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

				call := lastYSFCall
				if call == "" {
					call = cfg.BridgeCall
				}
				if ts.Sub(lastYSFTime) > 30*time.Second && cfg.BridgeCall != "" {
					call = cfg.BridgeCall
				}
				ensureYSFStation(stations24hMap, call)
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

			// Inicio de transmisión DMR en MMDVM_Bridge
			if m := dmrBeginTxRe.FindStringSubmatch(line); len(m) == 5 {
				ts, err := parseLogTime(m[1], now.Location())
				if err != nil {
					continue
				}
				call := strings.TrimSpace(m[4])
				if call == "" || call == "N0CALL" {
					call = strings.TrimSpace(m[2])
				}
				// Si viene inmediatamente después de tráfico YSF con el mismo indicativo (puente YSF->DMR), lo marcamos
				fromYSF := lastYSFCall != "" && strings.EqualFold(lastYSFCall, call) && ts.Sub(lastYSFTime) <= 3*time.Second
				pendingDMRTx = &activeDMRTx{
					callsign: call,
					srcID:    strings.TrimSpace(m[2]),
					dstTG:    strings.TrimSpace(m[3]),
					start:    ts,
					fromYSF:  fromYSF,
				}
				continue
			}

			// Fin de transmisión DMR en MMDVM_Bridge
			if m := dmrTxOffRe.FindStringSubmatch(line); len(m) == 3 {
				ts, err := parseLogTime(m[1], now.Location())
				if err != nil || ts.Before(cutoff24h) {
					pendingDMRTx = nil
					continue
				}
				frames, _ := strconv.ParseFloat(m[2], 64)
				// Cada frame DMR equivale a 60 ms (0.06 s)
				secs := frames * 0.06

				call := cfg.BridgeCall
				tg := cfg.TGIFTalkgroup
				isBridgeRelay := true

				if pendingDMRTx != nil {
					call = pendingDMRTx.callsign
					if pendingDMRTx.dstTG != "" {
						tg = pendingDMRTx.dstTG
					}
					if !pendingDMRTx.start.IsZero() && ts.Sub(pendingDMRTx.start) > 0 && ts.Sub(pendingDMRTx.start) < 10*time.Minute {
						secs = ts.Sub(pendingDMRTx.start).Seconds()
					}
					isBridgeRelay = pendingDMRTx.fromYSF
					pendingDMRTx = nil
				}

				// Contabilizar en DMR activo y tabla DMR si no es un simple rebote de ráfaga YSF
				if !isBridgeRelay && secs > 0.2 {
					durStr := formatDurationSecs(secs)
					agoStr := formatAgo(now.Sub(ts))

					dmrTX = append(dmrTX, TXEntry{
						Callsign: call,
						Duration: durStr,
						Ago:      agoStr,
					})
					combined = append(combined, CombinedActivity24h{
						Time:     ts.Format("15:04:05"),
						Mode:     "DMR",
						Callsign: call,
						Duration: durStr,
						Detail:   "TG " + tg,
					})

					agg, ok := dmr24hMap[call]
					if !ok {
						agg = &dmrAgg{callsign: call}
						dmr24hMap[call] = agg
					}
					agg.times++
					agg.totalSecs += secs
					if ts.After(agg.lastSeen) {
						agg.lastSeen = ts
					}
				}
				continue
			}
		}
		_ = f.Close()
	}

	// Actualizar estados de estaciones YSF según último snapshot activo
	activeNowSet := make(map[string]struct{}, len(currentSnapshotList))
	bridgeOnline := false
	for _, item := range currentSnapshotList {
		activeNowSet[item.Callsign] = struct{}{}
		if item.Type == "Puente local" {
			bridgeOnline = true
		}
	}
	for call, st := range stations24hMap {
		if _, online := activeNowSet[call]; !online {
			st.Status = "Desconectado"
		}
	}

	reverseTX(ysfTX)
	reverseTX(dmrTX)
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

	bridgeStatus := "Inactivo"
	if bridgeOnline {
		bridgeStatus = "En línea"
	}

	lastUpdateStr := "—"
	if !currentSnapshotTime.IsZero() {
		lastUpdateStr = currentSnapshotTime.Format("03:04 PM")
	}

	resp := DashboardStatusResponse{
		Header: HeaderInfo{
			Title:             cfg.Title,
			Subtitle:          fmt.Sprintf("Reflector YSF %s y TGIF DMR TG %s · se actualiza cada 5 segundos · última lectura %s", cfg.YSFReflector, cfg.TGIFTalkgroup, lastUpdateStr),
			Badge:             "Reflector + TGIF DMR",
			LocalBridgeStatus: bridgeStatus,
			LastUpdate:        lastUpdateStr,
		},
		Stats: StatsInfo{
			YSFConnectedNow: len(currentSnapshotList),
			YSFTx24h:        len(ysfTX),
			DMRTx24h:        len(dmrTX),
			Stations24h:     len(uniqueCallsigns),
		},
		YSFLastTX:        limitTX(ysfTX, 25),
		DMRLastTX:        limitTX(dmrTX, 25),
		DMRActive24h:     dmrActiveList,
		YSFStations24h:   ysfStations,
		YSFConnectedList: currentSnapshotList,
		Combined24h:      limitCombined(combined, 50),
	}

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
		Callsign: tx.callsign,
		Duration: durStr,
		Ago:      agoStr,
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

func candidateLogFiles(dir, prefix string, now time.Time) []string {
	return candidateLogFilesAnyPrefix(dir, []string{prefix}, now)
}

func candidateLogFilesAnyPrefix(dir string, prefixes []string, now time.Time) []string {
	var files []string
	seen := make(map[string]struct{})
	for _, offset := range []int{-1, 0} {
		d := now.AddDate(0, 0, offset)
		for _, pfx := range prefixes {
			name := fmt.Sprintf("%s-%s.log", pfx, d.Format("2006-01-02"))
			full := filepath.Join(dir, name)
			if _, ok := seen[full]; ok {
				continue
			}
			if _, err := os.Stat(full); err == nil {
				seen[full] = struct{}{}
				files = append(files, full)
			}
		}
	}
	return files
}

func parseLogTime(raw string, loc *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 19 {
		return time.ParseInLocation("2006-01-02 15:04:05.000", raw[:23], loc)
	}
	return time.ParseInLocation("2006-01-02 15:04:05", raw, loc)
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

func reverseTX(s []TXEntry) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func limitTX(s []TXEntry, max int) []TXEntry {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func limitCombined(s []CombinedActivity24h, max int) []CombinedActivity24h {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
