package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	echoLinkRetryInterval = 12 * time.Second
	maxEchoLinkRetries    = 32
)

type echoLinkPermanent struct {
	Target        string     `json:"target"`
	Resolved      string     `json:"resolved,omitempty"`
	State         string     `json:"state"`
	Attempts      int        `json:"attempts"`
	LastAttempt   *time.Time `json:"lastAttempt,omitempty"`
	LastResult    string     `json:"lastResult,omitempty"`
	LastConnected *time.Time `json:"lastConnected,omitempty"`
}

type echoLinkRetry struct {
	echoLinkPermanent
	cancel context.CancelFunc
	done   chan struct{}
}

func (retry *echoLinkRetry) connectionTarget() string {
	if retry.Resolved != "" {
		return retry.Resolved
	}
	return retry.Target
}

func (s *ReflectorService) hasEchoLinkRetry(target string) bool {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	_, exists := s.retries[normalizeEchoLinkCallsign(target)]
	return exists
}

// RestoreEchoLinkConnections loads saved destinations before serving requests.
func (s *ReflectorService) RestoreEchoLinkConnections() error {
	path := s.cfg.EchoLinkPermanentFile
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("leer conexiones permanentes: %w", err)
	}
	var entries []echoLinkPermanent
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("decodificar conexiones permanentes: %w", err)
	}
	if len(entries) > maxEchoLinkRetries {
		return fmt.Errorf("demasiadas conexiones permanentes: %d", len(entries))
	}
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	for _, entry := range entries {
		key := normalizeEchoLinkCallsign(entry.Target)
		if !echoLinkTargetPattern.MatchString(entry.Target) || strings.EqualFold(entry.Target, "ALL") ||
			(entry.Resolved != "" && !echoLinkTargetPattern.MatchString(entry.Resolved)) {
			return fmt.Errorf("destino permanente inválido: %q", entry.Target)
		}
		if _, exists := s.retries[key]; exists {
			return fmt.Errorf("destino permanente duplicado: %q", entry.Target)
		}
		s.retries[key] = &echoLinkRetry{echoLinkPermanent: entry}
	}
	for key, retry := range s.retries {
		s.launchEchoLinkRetryLocked(key, retry)
	}
	return nil
}

func (s *ReflectorService) startEchoLinkPermanent(target string) (string, error) {
	key := normalizeEchoLinkCallsign(target)
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	if _, exists := s.retries[key]; exists {
		return "", errors.New("ya existe una conexión permanente para ese destino")
	}
	if len(s.retries) >= maxEchoLinkRetries {
		return "", fmt.Errorf("se alcanzó el máximo de %d conexiones permanentes", maxEchoLinkRetries)
	}
	retry := &echoLinkRetry{echoLinkPermanent: echoLinkPermanent{Target: target, State: "comprobando"}}
	s.retries[key] = retry
	if err := s.saveEchoLinkPermanentLocked(); err != nil {
		delete(s.retries, key)
		return "", err
	}
	s.retryStorageErr = nil
	s.launchEchoLinkRetryLocked(key, retry)
	return fmt.Sprintf("Conexión permanente con %s programada en segundo plano.", target), nil
}

func (s *ReflectorService) launchEchoLinkRetryLocked(key string, retry *echoLinkRetry) {
	ctx, cancel := context.WithCancel(context.Background())
	retry.cancel = cancel
	retry.done = make(chan struct{})
	go s.retryEchoLinkConnect(ctx, key, retry)
}

var echoLinkResolvedTarget = regexp.MustCompile(`(?i)(?:connecting|reconnecting|already connected) to "([^"]+)"`)

func (s *ReflectorService) retryEchoLinkConnect(ctx context.Context, key string, retry *echoLinkRetry) {
	defer close(retry.done)
	ticker := time.NewTicker(s.retryInterval)
	defer ticker.Stop()
	for {
		s.checkEchoLinkPermanent(ctx, key, retry)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *ReflectorService) checkEchoLinkPermanent(ctx context.Context, key string, retry *echoLinkRetry) {
	s.mu.RLock()
	command := strings.TrimSpace(s.cfg.EchoLinkCommand)
	s.mu.RUnlock()
	if command == "" {
		command = "tlbcmd"
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}
	s.retryMu.Lock()
	target := retry.connectionTarget()
	s.retryMu.Unlock()
	usersArgs := append(append([]string{}, parts[1:]...), "users")
	users, usersErr := s.runEchoLinkCommand(ctx, "users", parts[0], usersArgs, parts[1:], EchoLinkActionRequest{})
	if ctx.Err() != nil {
		return
	}
	if usersErr == nil {
		for _, station := range parseEchoLinkUsers(users) {
			if strings.EqualFold(station.User, target) && strings.Contains(station.Attribute, "P") {
				state := "conectado"
				if strings.Contains(station.Attribute, "x") {
					state = "esperando"
				}
				s.updateEchoLinkPermanent(key, retry, func(entry *echoLinkPermanent) bool {
					if entry.State == state {
						return false
					}
					entry.State = state
					if state == "conectado" {
						now := time.Now()
						entry.LastConnected = &now
					}
					return true
				})
				return
			}
		}
	}

	args := append(append([]string{}, parts[1:]...), "connect", retry.Target)
	output, err := s.runEchoLinkCommand(ctx, "connect", parts[0], args, parts[1:], EchoLinkActionRequest{Callsign: retry.Target})
	if ctx.Err() != nil {
		return
	}
	s.updateEchoLinkPermanent(key, retry, func(entry *echoLinkPermanent) bool {
		now := time.Now()
		entry.LastAttempt = &now
		entry.Attempts++
		if match := echoLinkResolvedTarget.FindStringSubmatch(output); len(match) == 2 && echoLinkTargetPattern.MatchString(match[1]) {
			entry.Resolved = match[1]
		}
		if err == nil || echoLinkIsAlreadyConnected(err) {
			entry.State = "esperando"
			entry.LastResult = "Comando aceptado por theLinkBox."
		} else {
			entry.State = "error"
			entry.LastResult = err.Error()
		}
		return true
	})
}

func (s *ReflectorService) updateEchoLinkPermanent(key string, retry *echoLinkRetry, update func(*echoLinkPermanent) bool) {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	if s.retries[key] != retry || !update(&retry.echoLinkPermanent) {
		return
	}
	if err := s.saveEchoLinkPermanentLocked(); err != nil {
		s.retryStorageErr = err
		log.Printf("EchoLink: no se pudo guardar el estado permanente: %v", err)
	} else {
		s.retryStorageErr = nil
	}
}

func (s *ReflectorService) cancelEchoLinkRetry(target string) (*echoLinkRetry, error) {
	key := normalizeEchoLinkCallsign(target)
	s.retryMu.Lock()
	retry := s.retries[key]
	if retry == nil {
		for candidateKey, candidate := range s.retries {
			if strings.EqualFold(candidate.Resolved, target) {
				key, retry = candidateKey, candidate
				break
			}
		}
	}
	if retry == nil {
		s.retryMu.Unlock()
		return nil, nil
	}
	delete(s.retries, key)
	if err := s.saveEchoLinkPermanentLocked(); err != nil {
		s.retries[key] = retry
		s.retryMu.Unlock()
		return nil, err
	}
	s.retryStorageErr = nil
	retry.cancel()
	s.retryMu.Unlock()
	<-retry.done
	return retry, nil
}

func (s *ReflectorService) echoLinkPermanentList() (string, error) {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	if s.retryStorageErr != nil {
		return "", s.retryStorageErr
	}
	entries := s.echoLinkPermanentEntriesLocked()
	data, err := json.Marshal(entries)
	return string(data), err
}

func (s *ReflectorService) echoLinkPermanentEntriesLocked() []echoLinkPermanent {
	entries := make([]echoLinkPermanent, 0, len(s.retries))
	for _, retry := range s.retries {
		entries = append(entries, retry.echoLinkPermanent)
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToUpper(entries[i].Target) < strings.ToUpper(entries[j].Target) })
	return entries
}

func (s *ReflectorService) echoLinkRetryList() string {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	lines := make([]string, 0, len(s.retries))
	for _, retry := range s.retries {
		if retry.State != "conectado" {
			lines = append(lines, fmt.Sprintf("%s %d", retry.Target, retry.Attempts))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func (s *ReflectorService) saveEchoLinkPermanentLocked() error {
	path := s.cfg.EchoLinkPermanentFile
	if path == "" {
		return nil
	}
	data, err := json.Marshal(s.echoLinkPermanentEntriesLocked())
	if err != nil {
		return fmt.Errorf("codificar conexiones permanentes: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("crear directorio de conexiones permanentes: %w", err)
	}
	file, err := os.CreateTemp(dir, ".echolink-permanent-*")
	if err != nil {
		return fmt.Errorf("crear archivo de conexiones permanentes: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("escribir conexiones permanentes: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sincronizar conexiones permanentes: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cerrar conexiones permanentes: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("guardar conexiones permanentes: %w", err)
	}
	return nil
}

// Close stops workers but retains the destinations for the next start.
func (s *ReflectorService) Close() {
	s.retryMu.Lock()
	retries := s.retries
	s.retries = make(map[string]*echoLinkRetry)
	for _, retry := range retries {
		retry.cancel()
	}
	s.retryMu.Unlock()
	for _, retry := range retries {
		<-retry.done
	}
}
