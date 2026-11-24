package controllers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

const (
	echoLinkRetryInterval = 12 * time.Second
	maxEchoLinkRetries    = 32
)

type echoLinkRetry struct {
	target   string
	cancel   context.CancelFunc
	attempts int
}

func (s *ReflectorService) hasEchoLinkRetry(target string) bool {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	_, exists := s.retries[strings.ToUpper(target)]
	return exists
}

func (s *ReflectorService) startEchoLinkRetry(target, name string, args, configuredArgs []string, request EchoLinkActionRequest) error {
	key := strings.ToUpper(target)
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	if _, exists := s.retries[key]; exists {
		return errors.New("ya hay reintentos en segundo plano para ese destino")
	}
	if len(s.retries) >= maxEchoLinkRetries {
		return fmt.Errorf("se alcanzó el máximo de %d conexiones permanentes pendientes", maxEchoLinkRetries)
	}
	ctx, cancel := context.WithCancel(context.Background())
	retry := &echoLinkRetry{target: target, cancel: cancel, attempts: 1}
	s.retries[key] = retry
	log.Printf("EchoLink reintento: destino=%q programado cada %s", target, s.retryInterval)
	go s.retryEchoLinkConnect(ctx, key, retry, name, append([]string{}, args...), configuredArgs, request)
	return nil
}

func (s *ReflectorService) retryEchoLinkConnect(ctx context.Context, key string, retry *echoLinkRetry, name string, args, configuredArgs []string, request EchoLinkActionRequest) {
	ticker := time.NewTicker(s.retryInterval)
	defer ticker.Stop()
	defer s.removeEchoLinkRetry(key, retry)
	for {
		select {
		case <-ctx.Done():
			log.Printf("EchoLink reintento: destino=%q cancelado tras %d intentos", retry.target, s.echoLinkRetryAttempts(retry))
			return
		case <-ticker.C:
		}
		s.retryMu.Lock()
		retry.attempts++
		attempt := retry.attempts
		s.retryMu.Unlock()
		log.Printf("EchoLink reintento: destino=%q intento=%d", retry.target, attempt)
		if _, err := s.runEchoLinkCommand(ctx, "connect", name, args, configuredArgs, request); err != nil {
			if ctx.Err() == nil {
				log.Printf("EchoLink reintento: destino=%q intento=%d falló: %v", retry.target, attempt, err)
			}
			continue
		}
		log.Printf("EchoLink reintento: destino=%q conectado en el intento %d", retry.target, attempt)
		return
	}
}

func (s *ReflectorService) echoLinkRetryAttempts(retry *echoLinkRetry) int {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	return retry.attempts
}

func (s *ReflectorService) removeEchoLinkRetry(key string, retry *echoLinkRetry) {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	if s.retries[key] == retry {
		delete(s.retries, key)
	}
	retry.cancel()
}

func (s *ReflectorService) cancelEchoLinkRetry(target string) bool {
	key := strings.ToUpper(target)
	s.retryMu.Lock()
	retry, exists := s.retries[key]
	if exists {
		delete(s.retries, key)
	}
	s.retryMu.Unlock()
	if exists {
		retry.cancel()
	}
	return exists
}

// echoLinkRetryList returns one "<target> <attempts>" line per pending retry.
func (s *ReflectorService) echoLinkRetryList() string {
	s.retryMu.Lock()
	lines := make([]string, 0, len(s.retries))
	for _, retry := range s.retries {
		lines = append(lines, fmt.Sprintf("%s %d", retry.target, retry.attempts))
	}
	s.retryMu.Unlock()
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// Close stops every pending background EchoLink retry.
func (s *ReflectorService) Close() {
	s.retryMu.Lock()
	retries := s.retries
	s.retries = make(map[string]*echoLinkRetry)
	s.retryMu.Unlock()
	for _, retry := range retries {
		retry.cancel()
	}
}
