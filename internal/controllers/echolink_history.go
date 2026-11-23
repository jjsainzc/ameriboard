package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type EchoLinkLastHeard struct {
	Callsign       string    `json:"callsign"`
	DisconnectedAt time.Time `json:"disconnectedAt"`
	Duration       string    `json:"duration"`
}

type echoLinkHistoryFile struct {
	Active    map[string]echoLinkActiveSession `json:"active"`
	LastHeard map[string]EchoLinkLastHeard     `json:"lastHeard"`
}

type echoLinkActiveSession struct {
	Callsign    string    `json:"callsign"`
	ConnectedAt time.Time `json:"connectedAt"`
}

type echoLinkHistoryStore struct {
	path string
	mu   sync.Mutex
}

func newEchoLinkHistoryStore(path string) *echoLinkHistoryStore {
	return &echoLinkHistoryStore{path: path}
}

func (s *echoLinkHistoryStore) Observe(stations []EchoLinkStation, now time.Time) ([]EchoLinkLastHeard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.read()
	if err != nil {
		return nil, err
	}

	current := make(map[string]EchoLinkStation, len(stations))
	for _, station := range stations {
		key := normalizeEchoLinkCallsign(station.User)
		if key != "" {
			current[key] = station
		}
	}

	changed := false
	for key, station := range current {
		if _, exists := state.Active[key]; !exists {
			state.Active[key] = echoLinkActiveSession{
				Callsign: station.User, ConnectedAt: now,
			}
			changed = true
		}
	}
	for key, active := range state.Active {
		if _, stillConnected := current[key]; stillConnected {
			continue
		}
		duration := now.Sub(active.ConnectedAt)
		if duration < 0 {
			duration = 0
		}
		state.LastHeard[key] = EchoLinkLastHeard{
			Callsign: active.Callsign, DisconnectedAt: now,
			Duration: formatEchoLinkDuration(duration),
		}
		delete(state.Active, key)
		changed = true
	}

	if changed {
		if err := s.write(state); err != nil {
			return echoLinkHistoryRows(state.LastHeard), err
		}
	}
	return echoLinkHistoryRows(state.LastHeard), nil
}

func (s *echoLinkHistoryStore) Read() ([]EchoLinkLastHeard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.read()
	if err != nil {
		return nil, err
	}
	return echoLinkHistoryRows(state.LastHeard), nil
}

func (s *echoLinkHistoryStore) read() (echoLinkHistoryFile, error) {
	state := echoLinkHistoryFile{
		Active:    make(map[string]echoLinkActiveSession),
		LastHeard: make(map[string]EchoLinkLastHeard),
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("leer historial de EchoLink: %w", err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("decodificar historial de EchoLink: %w", err)
	}
	if state.Active == nil {
		state.Active = make(map[string]echoLinkActiveSession)
	}
	if state.LastHeard == nil {
		state.LastHeard = make(map[string]EchoLinkLastHeard)
	}
	return state, nil
}

func (s *echoLinkHistoryStore) write(state echoLinkHistoryFile) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("crear directorio del historial de EchoLink: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("codificar historial de EchoLink: %w", err)
	}
	file, err := os.CreateTemp(directory, ".echolink-history-*")
	if err != nil {
		return fmt.Errorf("crear archivo temporal del historial de EchoLink: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("escribir historial de EchoLink: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sincronizar historial de EchoLink: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cerrar historial de EchoLink: %w", err)
	}
	if err := os.Rename(file.Name(), s.path); err != nil {
		return fmt.Errorf("guardar historial de EchoLink: %w", err)
	}
	return nil
}

func echoLinkHistoryRows(history map[string]EchoLinkLastHeard) []EchoLinkLastHeard {
	rows := make([]EchoLinkLastHeard, 0, len(history))
	for _, entry := range history {
		rows = append(rows, entry)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DisconnectedAt.Equal(rows[j].DisconnectedAt) {
			return rows[i].Callsign < rows[j].Callsign
		}
		return rows[i].DisconnectedAt.After(rows[j].DisconnectedAt)
	})
	return rows
}

func normalizeEchoLinkCallsign(callsign string) string {
	return strings.ToUpper(strings.TrimSpace(callsign))
}

func formatEchoLinkDuration(duration time.Duration) string {
	totalSeconds := int(duration.Seconds())
	hours := totalSeconds / 3600
	minutes := totalSeconds % 3600 / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
