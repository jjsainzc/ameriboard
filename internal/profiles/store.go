package profiles

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

type Profile struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	URL      string         `json:"url"`
	Token    string         `json:"token"`
	Password string         `json:"password,omitempty"`
	Visible  bool           `json:"visible"`
	Header   *ProfileHeader `json:"header,omitempty"`
}

type ProfileHeader struct {
	Title         string `json:"title"`
	Subtitle      string `json:"subtitle"`
	Badge         string `json:"badge"`
	DVSwitchTitle string `json:"dvSwitchTitle,omitempty"`
	NXDNTitle     string `json:"nxdnTitle,omitempty"`
}

const DefaultDVSwitchTitle = "Actividad digital vía DVSwitch"
const DefaultNXDNTitle = "Últimos escuchados NXDN"

type MainSettings struct {
	Title         string `json:"title"`
	Subtitle      string `json:"subtitle"`
	DVSwitchTitle string `json:"dvSwitchTitle"`
	NXDNTitle     string `json:"nxdnTitle"`
}

type Store struct {
	dir string
	mu  sync.RWMutex
}

var errInvalidProfile = errors.New("perfil inválido")
var errInvalidPassword = errors.New("contraseña inválida")
var errInvalidMainSettings = errors.New("configuración principal inválida")

func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) LoadMainSettings(defaults MainSettings) (MainSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(filepath.Join(s.dir, "dashboard-settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return MainSettings{}, err
	}
	var settings MainSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return MainSettings{}, err
	}
	if settings.DVSwitchTitle == "" {
		settings.DVSwitchTitle = defaults.DVSwitchTitle
	}
	if err := validateMainSettings(&settings); err != nil {
		return MainSettings{}, err
	}
	return settings, nil
}

func (s *Store) SaveMainSettings(settings MainSettings) (MainSettings, error) {
	if err := validateMainSettings(&settings); err != nil {
		return MainSettings{}, err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return MainSettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.write("dashboard-settings.json", data); err != nil {
		return MainSettings{}, err
	}
	return settings, nil
}

func validateMainSettings(settings *MainSettings) error {
	settings.Title = strings.TrimSpace(settings.Title)
	settings.Subtitle = strings.TrimSpace(settings.Subtitle)
	settings.DVSwitchTitle = strings.TrimSpace(settings.DVSwitchTitle)
	settings.NXDNTitle = strings.TrimSpace(settings.NXDNTitle)
	if settings.DVSwitchTitle == "" {
		settings.DVSwitchTitle = DefaultDVSwitchTitle
	}
	if settings.NXDNTitle == "" {
		settings.NXDNTitle = DefaultNXDNTitle
	}
	if settings.Title == "" || utf8.RuneCountInString(settings.Title) > 200 ||
		utf8.RuneCountInString(settings.Subtitle) > 500 ||
		utf8.RuneCountInString(settings.DVSwitchTitle) > 200 ||
		utf8.RuneCountInString(settings.NXDNTitle) > 200 {
		return errInvalidMainSettings
	}
	return nil
}

func randomValue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) write(name string, data []byte) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(s.dir, ".ameriboard-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(s.dir, name))
}

func (s *Store) writeNew(name string, data []byte) error {
	file, err := os.CreateTemp(s.dir, ".ameriboard-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Link(file.Name(), filepath.Join(s.dir, name))
}

// GenerateCredential replaces the credential and returns its new secret once.
func (s *Store) GenerateCredential(kind string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	secret, err := randomValue()
	if err != nil {
		return "", err
	}
	switch kind {
	case "admin":
		hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		err = s.write("admin.hash", hash)
	case "site":
		hash := sha256.Sum256([]byte(secret))
		err = s.write("site.hash", []byte(hex.EncodeToString(hash[:])))
	default:
		return "", errors.New("tipo de credencial inválido")
	}
	if err != nil {
		return "", err
	}
	return secret, nil
}

func (s *Store) setupRequired() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := os.Stat(filepath.Join(s.dir, "admin.hash"))
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

func (s *Store) setupAdmin(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(filepath.Join(s.dir, "admin.hash")); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(password) < 8 || len(password) > 72 {
		return errInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	if err := s.writeNew("admin.hash", hash); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.dir, "setup.hash")); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("No se pudo eliminar el código inicial ya utilizado: %v", err)
	}
	return nil
}

func (s *Store) credential(kind string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return os.ReadFile(filepath.Join(s.dir, kind+".hash"))
}

func (s *Store) list() ([]Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readProfiles()
}

func (s *Store) readProfiles() ([]Profile, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "profiles.json"))
	if errors.Is(err, os.ErrNotExist) {
		return []Profile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func validProfile(p Profile) error {
	if len(strings.TrimSpace(p.Name)) == 0 || len(p.Name) > 100 {
		return fmt.Errorf("%w: nombre inválido", errInvalidProfile)
	}
	u, err := url.Parse(p.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("%w: la dirección debe ser HTTP o HTTPS y no incluir credenciales, consulta ni fragmento", errInvalidProfile)
	}
	if len(p.URL) > 2048 || (p.Password == "" && (len(p.Token) < 32 || len(p.Token) > 512 || strings.ContainsAny(p.Token, "\r\n"))) ||
		(p.Password != "" && (len(p.Password) < 8 || len(p.Password) > 72)) {
		return fmt.Errorf("%w: dirección o contraseña inválida", errInvalidProfile)
	}
	if p.Header != nil && (utf8.RuneCountInString(p.Header.Title) > 200 ||
		utf8.RuneCountInString(p.Header.Subtitle) > 500 || utf8.RuneCountInString(p.Header.Badge) > 100 ||
		utf8.RuneCountInString(p.Header.DVSwitchTitle) > 200 ||
		utf8.RuneCountInString(p.Header.NXDNTitle) > 200) {
		return fmt.Errorf("%w: cabecera demasiado larga", errInvalidProfile)
	}
	return nil
}

func (s *Store) save(p Profile, id string) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles, err := s.readProfiles()
	if err != nil {
		return Profile{}, err
	}
	index := -1
	if id != "" {
		for i := range profiles {
			if profiles[i].ID == id {
				index = i
				if p.Password != "" {
					p.Token = ""
				} else if p.Token == "" {
					p.Token = profiles[i].Token
					p.Password = profiles[i].Password
				}
				if p.Header == nil {
					p.Header = profiles[i].Header
				}
				break
			}
		}
		if index < 0 {
			return Profile{}, os.ErrNotExist
		}
	}
	p.Name = strings.TrimSpace(p.Name)
	p.URL = strings.TrimRight(strings.TrimSpace(p.URL), "/")
	if p.Header != nil {
		p.Header.Title = strings.TrimSpace(p.Header.Title)
		p.Header.Subtitle = strings.TrimSpace(p.Header.Subtitle)
		p.Header.Badge = strings.TrimSpace(p.Header.Badge)
		p.Header.DVSwitchTitle = strings.TrimSpace(p.Header.DVSwitchTitle)
		p.Header.NXDNTitle = strings.TrimSpace(p.Header.NXDNTitle)
	}
	if err := validProfile(p); err != nil {
		return Profile{}, err
	}
	u, err := url.Parse(p.URL)
	if err != nil {
		return Profile{}, fmt.Errorf("%w: dirección inválida", errInvalidProfile)
	}
	if u.Path == "" && (index < 0 || p.URL != strings.TrimRight(profiles[index].URL, "/")) {
		p.URL += "/ameriboard"
		if err := validProfile(p); err != nil {
			return Profile{}, err
		}
	}
	if index < 0 {
		p.ID, err = randomValue()
		if err != nil {
			return Profile{}, err
		}
		profiles = append(profiles, p)
	} else {
		p.ID = id
		profiles[index] = p
	}
	data, err := json.Marshal(profiles)
	if err != nil {
		return Profile{}, err
	}
	if err := s.write("profiles.json", data); err != nil {
		return Profile{}, fmt.Errorf("guardar perfiles: %w", err)
	}
	return p, nil
}

func (s *Store) delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles, err := s.readProfiles()
	if err != nil {
		return err
	}
	for i := range profiles {
		if profiles[i].ID == id {
			profiles = append(profiles[:i], profiles[i+1:]...)
			data, err := json.Marshal(profiles)
			if err != nil {
				return err
			}
			return s.write("profiles.json", data)
		}
	}
	return os.ErrNotExist
}
