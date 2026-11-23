package controllers

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type DVSwitchBridge struct {
	ModeA string `json:"modeA"`
	ModeB string `json:"modeB"`
}

type bridgePorts struct {
	tx int
	rx int
}

var bridgeModes = map[string]string{
	"DMR": "DMR", "DSTAR": "D-Star", "NXDN": "NXDN", "P25": "P25", "YSF": "YSF",
}

func readDVSwitchBridges(root string) ([]DVSwitchBridge, error) {
	bridges := make([]DVSwitchBridge, 0)
	if root == "" {
		return bridges, nil
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return bridges, nil
	}
	if err != nil {
		return bridges, fmt.Errorf("leer directorio de puentes: %w", err)
	}
	seen := make(map[DVSwitchBridge]struct{})
	var failures []error
	for _, entry := range entries {
		if !strings.HasPrefix(strings.ToLower(entry.Name()), "mmdvm_bridge") {
			continue
		}
		if !entry.IsDir() {
			if entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			info, err := os.Stat(filepath.Join(root, entry.Name()))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("examinar %s: %w", entry.Name(), err))
				continue
			}
			if !info.IsDir() {
				continue
			}
		}
		path := filepath.Join(root, entry.Name(), "DVSwitch.ini")
		file, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("abrir %s: %w", path, err))
			continue
		}
		found, readErr := parseDVSwitchBridges(file)
		closeErr := file.Close()
		if readErr != nil {
			failures = append(failures, fmt.Errorf("leer %s: %w", path, readErr))
			continue
		}
		if closeErr != nil {
			failures = append(failures, fmt.Errorf("cerrar %s: %w", path, closeErr))
			continue
		}
		for _, bridge := range found {
			if _, exists := seen[bridge]; !exists {
				seen[bridge] = struct{}{}
				bridges = append(bridges, bridge)
			}
		}
	}
	sort.Slice(bridges, func(i, j int) bool {
		if bridges[i].ModeA != bridges[j].ModeA {
			return bridges[i].ModeA < bridges[j].ModeA
		}
		return bridges[i].ModeB < bridges[j].ModeB
	})
	return bridges, errors.Join(failures...)
}

func parseDVSwitchBridges(r io.Reader) ([]DVSwitchBridge, error) {
	ports := make(map[string]bridgePorts)
	section := ""
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), ";", 2)[0])
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToUpper(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		mode, supported := bridgeModes[section]
		if !supported {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		if !ok || (key != "txport" && key != "rxport") {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || port < 0 || port > 65535 {
			return nil, fmt.Errorf("puerto %s inválido en sección %s", key, section)
		}
		pair := ports[mode]
		if key == "txport" {
			pair.tx = port
		} else {
			pair.rx = port
		}
		ports[mode] = pair
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	modes := make([]string, 0, len(ports))
	for mode := range ports {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	bridges := make([]DVSwitchBridge, 0)
	for i, modeA := range modes {
		a := ports[modeA]
		if a.tx == 0 || a.rx == 0 || a.tx == a.rx {
			continue
		}
		for _, modeB := range modes[i+1:] {
			b := ports[modeB]
			if a.tx == b.rx && a.rx == b.tx {
				bridges = append(bridges, DVSwitchBridge{ModeA: modeA, ModeB: modeB})
			}
		}
	}
	return bridges, nil
}
