package controllers

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type NXDNReflectorData struct {
	Available bool              `json:"available"`
	TG        string            `json:"tg"`
	Port      string            `json:"port"`
	Connected []YSFConnectedNow `json:"connected"`
	LastHeard []TXEntry         `json:"lastHeard"`
}

var (
	nxdnTimestampRe = regexp.MustCompile(`^M:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+(.*)$`)
	nxdnStationRe   = regexp.MustCompile(`^([A-Z0-9_/\-]+)\s*:\s+((\d+\.\d+\.\d+\.\d+):\d+)\s+\d+/\d+`)
	nxdnChangeRe    = regexp.MustCompile(`^(Adding|Removing)\s+([A-Z0-9_/\-]+)\s+\(((\d+\.\d+\.\d+\.\d+):\d+)\)`)
	nxdnTxRe        = regexp.MustCompile(`^Transmission from\s+(\S+)\s+(?:at\s+(\S+)|on\s+(Icom|Kenwood) Network)\s+to\s+(?:TG\s+)?(\S+)`)
)

type nxdnEvent struct {
	at   time.Time
	text string
}

func readNXDNReflectorStatus(iniPath, logDir string, now time.Time) (NXDNReflectorData, error) {
	result := NXDNReflectorData{Connected: []YSFConnectedNow{}, LastHeard: []TXEntry{}}
	ini, err := os.Open(iniPath)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("abrir configuración NXDN: %w", err)
	}
	section := ""
	scanner := bufio.NewScanner(ini)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
		if section == "general" && key == "tg" {
			result.TG = value
		}
		if section == "network" && key == "port" {
			result.Port = value
		}
	}
	scanErr := scanner.Err()
	closeErr := ini.Close()
	if scanErr != nil {
		return result, fmt.Errorf("leer configuración NXDN: %w", scanErr)
	}
	if closeErr != nil {
		return result, fmt.Errorf("cerrar configuración NXDN: %w", closeErr)
	}

	files, err := candidateLogFiles(logDir, "NXDNReflector", now)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("buscar registros NXDN: %w", err)
	}
	exact := filepath.Join(logDir, "NXDNReflector.log")
	if _, err := os.Stat(exact); err == nil {
		files = append(files, exact)
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("examinar registro NXDN: %w", err)
	}
	if len(files) == 0 {
		return result, nil
	}
	events := make([]nxdnEvent, 0)
	seen := make(map[string]struct{})
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return result, fmt.Errorf("abrir registro NXDN: %w", err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			match := nxdnTimestampRe.FindStringSubmatch(line)
			if len(match) != 3 {
				continue
			}
			at, err := parseLogTime(match[1], now.Location())
			if err != nil {
				continue
			}
			if _, duplicate := seen[line]; duplicate {
				continue
			}
			seen[line] = struct{}{}
			events = append(events, nxdnEvent{at: at, text: strings.TrimSpace(match[2])})
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return result, fmt.Errorf("leer registro NXDN: %w", scanErr)
		}
		if closeErr != nil {
			return result, fmt.Errorf("cerrar registro NXDN: %w", closeErr)
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	connected := make(map[string]YSFConnectedNow)
	inSnapshot := false
	var pending *activeYSFTx
	var lastHeard []TXEntry
	cutoff := now.Add(-24 * time.Hour)
	addTX := func(tx *activeYSFTx, end time.Time, duration string) {
		if !end.Before(cutoff) && !end.After(now) {
			lastHeard = append(lastHeard, TXEntry{Callsign: tx.callsign, Gateway: tx.gateway,
				Target: tx.target, Duration: duration, Ago: formatAgo(now.Sub(end)), timestamp: end})
		}
	}
	for _, event := range events {
		if event.at.After(now) {
			continue
		}
		if event.text == "Currently linked repeaters:" {
			connected = make(map[string]YSFConnectedNow)
			inSnapshot = true
			continue
		}
		if event.text == "No repeaters linked" {
			connected = make(map[string]YSFConnectedNow)
			inSnapshot = false
			continue
		}
		if inSnapshot {
			if match := nxdnStationRe.FindStringSubmatch(event.text); len(match) == 4 {
				connected[match[1]+"@"+match[2]] = YSFConnectedNow{Callsign: match[1], IP: match[2],
					Type: ysfGatewayType(match[3]), Since: event.at.Format("03:04 PM")}
				continue
			}
			inSnapshot = false
		}
		if match := nxdnChangeRe.FindStringSubmatch(event.text); len(match) == 5 {
			key := match[2] + "@" + match[3]
			if match[1] == "Adding" {
				connected[key] = YSFConnectedNow{Callsign: match[2], IP: match[3],
					Type: ysfGatewayType(match[4]), Since: event.at.Format("03:04 PM")}
			} else {
				delete(connected, key)
			}
			continue
		}
		if match := nxdnTxRe.FindStringSubmatch(event.text); len(match) == 5 {
			if pending != nil {
				addTX(pending, pending.start, "—")
			}
			gateway := match[2]
			if gateway == "" {
				gateway = match[3] + " Network"
			}
			pending = &activeYSFTx{callsign: match[1], gateway: gateway, target: "TG " + match[4], start: event.at}
			continue
		}
		if pending != nil && event.text == "Received end of transmission" {
			addTX(pending, event.at, formatDurationSecs(event.at.Sub(pending.start).Seconds()))
			pending = nil
		} else if pending != nil && event.text == "Network watchdog has expired" {
			addTX(pending, pending.start, "—")
			pending = nil
		}
	}
	if pending != nil {
		addTX(pending, pending.start, "—")
	}
	for _, station := range connected {
		result.Connected = append(result.Connected, station)
	}
	sort.Slice(result.Connected, func(i, j int) bool {
		if result.Connected[i].Callsign == result.Connected[j].Callsign {
			return result.Connected[i].IP < result.Connected[j].IP
		}
		return result.Connected[i].Callsign < result.Connected[j].Callsign
	})
	sort.SliceStable(lastHeard, func(i, j int) bool { return lastHeard[i].timestamp.After(lastHeard[j].timestamp) })
	result.LastHeard = limitUniqueTX(lastHeard, len(lastHeard))
	result.Available = true
	return result, nil
}
