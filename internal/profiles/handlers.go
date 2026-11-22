package profiles

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type session struct {
	expires time.Time
	hash    string
}

type Handler struct {
	store          *Store
	client         *http.Client
	cookiePath     string
	mu             sync.Mutex
	sessions       map[string]session
	login          *rate.Limiter
	remoteFailures *rate.Limiter
}

var nonPublicNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range nonPublicNetworks {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func NewHandler(store *Store) *Handler {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = publicDial
	return &Handler{
		store: store,
		client: &http.Client{Transport: transport, Timeout: 4 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		sessions:       make(map[string]session),
		login:          rate.NewLimiter(rate.Every(time.Minute/5), 5),
		remoteFailures: rate.NewLimiter(rate.Every(time.Minute/5), 5),
	}
}

func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	var dialer net.Dialer
	for _, ip := range ips {
		ip = ip.Unmap()
		if !publicIP(ip) {
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, errors.New("el sitio no tiene una dirección pública accesible")
}

func (h *Handler) Register(group *echo.Group, basePath string, status echo.HandlerFunc) {
	h.cookiePath = strings.TrimRight(basePath, "/") + "/"
	group.GET("/api/profiles", h.publicProfiles)
	group.GET("/api/profiles/:id/status", h.profileStatus)
	group.POST("/api/admin/login", h.loginAdmin)
	group.GET("/api/admin/setup", h.setupStatus)
	group.POST("/api/admin/setup", h.setupAdmin)
	group.POST("/api/admin/logout", h.logoutAdmin)
	group.GET("/api/admin/profiles", h.adminProfiles)
	group.POST("/api/admin/profiles", h.createProfile)
	group.PUT("/api/admin/profiles/:id", h.updateProfile)
	group.DELETE("/api/admin/profiles/:id", h.deleteProfile)
	group.GET("/api/remote/status", func(c echo.Context) error {
		if !secureRequest(c.Request()) {
			return echo.NewHTTPError(http.StatusForbidden, "Se requiere HTTPS")
		}
		if _, password, ok := c.Request().BasicAuth(); ok {
			hash, err := h.store.credential("admin")
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return echo.ErrUnauthorized
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "No se puede leer la credencial de administrador").SetInternal(err)
			}
			if len(password) > 72 || bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
				if !h.remoteFailures.Allow() {
					return echo.NewHTTPError(http.StatusTooManyRequests, "Demasiados intentos; inténtelo más tarde")
				}
				return echo.ErrUnauthorized
			}
			return status(c)
		}
		hash, err := h.store.credential("site")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return echo.NewHTTPError(http.StatusUnauthorized, "Acceso remoto no configurado")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "No se puede leer la credencial remota").SetInternal(err)
		}
		value, ok := strings.CutPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
		if !ok {
			return echo.ErrUnauthorized
		}
		actual := sha256.Sum256([]byte(value))
		expected, err := hex.DecodeString(string(hash))
		if err != nil || len(expected) != sha256.Size || subtle.ConstantTimeCompare(actual[:], expected) != 1 {
			return echo.ErrUnauthorized
		}
		return status(c)
	})
}

func (h *Handler) setupStatus(c echo.Context) error {
	required, err := h.store.setupRequired()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "No se puede consultar el administrador").SetInternal(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, map[string]bool{"required": required})
}

func (h *Handler) setupAdmin(c echo.Context) error {
	if !adminRequest(c) {
		return echo.ErrForbidden
	}
	if !h.login.Allow() {
		return echo.NewHTTPError(http.StatusTooManyRequests, "Demasiados intentos; inténtelo más tarde")
	}
	var request struct {
		Password string `json:"password"`
	}
	if err := decodeBody(c, &request); err != nil {
		return echo.ErrBadRequest
	}
	if err := h.store.setupAdmin(request.Password); err != nil {
		switch {
		case errors.Is(err, os.ErrExist):
			return echo.NewHTTPError(http.StatusConflict, "Ya existe un administrador")
		case errors.Is(err, errInvalidPassword):
			return echo.NewHTTPError(http.StatusBadRequest, "La contraseña debe tener entre 8 y 72 bytes")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "No se pudo crear el administrador").SetInternal(err)
		}
	}
	hash, err := h.store.credential("admin")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Administrador creado, inicia sesión").SetInternal(err)
	}
	return h.startSession(c, hash)
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func adminRequest(c echo.Context) bool {
	r := c.Request()
	origin, err := url.Parse(r.Header.Get("Origin"))
	return secureRequest(r) && err == nil && origin.Scheme == "https" &&
		strings.EqualFold(origin.Host, r.Host) && r.Header.Get("X-Ameriboard-Request") == "1"
}

func (h *Handler) authenticated(c echo.Context) bool {
	cookie, err := c.Cookie("ameriboard_admin")
	if err != nil {
		return false
	}
	hash, err := h.store.credential("admin")
	if err != nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[cookie.Value]
	if !ok || time.Now().After(s.expires) || s.hash != string(hash) {
		delete(h.sessions, cookie.Value)
		return false
	}
	return true
}

func (h *Handler) loginAdmin(c echo.Context) error {
	if !adminRequest(c) {
		return echo.ErrForbidden
	}
	if !h.login.Allow() {
		return echo.NewHTTPError(http.StatusTooManyRequests, "Demasiados intentos; inténtelo más tarde")
	}
	var request struct {
		Password string `json:"password"`
	}
	if err := decodeBody(c, &request); err != nil || len(request.Password) > 72 {
		return echo.ErrBadRequest
	}
	hash, err := h.store.credential("admin")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "Administrador no configurado")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "No se puede leer la credencial de administrador").SetInternal(err)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(request.Password)) != nil {
		return echo.ErrUnauthorized
	}
	return h.startSession(c, hash)
}

func (h *Handler) startSession(c echo.Context, hash []byte) error {
	id, err := randomValue()
	if err != nil {
		return err
	}
	now := time.Now()
	h.mu.Lock()
	for key, value := range h.sessions {
		if now.After(value.expires) {
			delete(h.sessions, key)
		}
	}
	h.sessions[id] = session{expires: now.Add(8 * time.Hour), hash: string(hash)}
	h.mu.Unlock()
	c.SetCookie(&http.Cookie{Name: "ameriboard_admin", Value: id, Path: h.cookiePath, HttpOnly: true,
		Secure: true, SameSite: http.SameSiteStrictMode})
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) logoutAdmin(c echo.Context) error {
	if !adminRequest(c) || !h.authenticated(c) {
		return echo.ErrUnauthorized
	}
	cookie, _ := c.Cookie("ameriboard_admin")
	h.mu.Lock()
	delete(h.sessions, cookie.Value)
	h.mu.Unlock()
	c.SetCookie(&http.Cookie{Name: "ameriboard_admin", Path: h.cookiePath, HttpOnly: true, Secure: true,
		SameSite: http.SameSiteStrictMode, MaxAge: -1})
	return c.NoContent(http.StatusNoContent)
}

type publicProfile struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	URL     string         `json:"url,omitempty"`
	Visible bool           `json:"visible"`
	Header  *ProfileHeader `json:"header,omitempty"`
}

func (h *Handler) profiles(c echo.Context, admin bool) error {
	profiles, err := h.store.list()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "No se pueden leer los perfiles").SetInternal(err)
	}
	result := make([]publicProfile, 0)
	for _, p := range profiles {
		if p.Visible || admin {
			item := publicProfile{ID: p.ID, Name: p.Name, Visible: p.Visible}
			if admin {
				item.URL = p.URL
				item.Header = p.Header
			}
			result = append(result, item)
		}
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) publicProfiles(c echo.Context) error { return h.profiles(c, h.authenticated(c)) }

func (h *Handler) adminProfiles(c echo.Context) error {
	if !h.authenticated(c) {
		return echo.ErrUnauthorized
	}
	return h.profiles(c, true)
}

func decodeBody(c echo.Context, target any) error {
	reader := http.MaxBytesReader(c.Response(), c.Request().Body, 16*1024)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("contenido adicional")
	}
	return nil
}

func (h *Handler) modify(c echo.Context, id string) error {
	if !adminRequest(c) || !h.authenticated(c) {
		return echo.ErrUnauthorized
	}
	var p Profile
	if err := decodeBody(c, &p); err != nil {
		return echo.ErrBadRequest
	}
	p, err := h.store.save(p, id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return echo.ErrNotFound
		}
		if errors.Is(err, errInvalidProfile) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "No se pudo guardar el perfil").SetInternal(err)
	}
	return c.JSON(http.StatusOK, publicProfile{ID: p.ID, Name: p.Name, URL: p.URL, Visible: p.Visible, Header: p.Header})
}

func (h *Handler) createProfile(c echo.Context) error { return h.modify(c, "") }
func (h *Handler) updateProfile(c echo.Context) error { return h.modify(c, c.Param("id")) }

func (h *Handler) deleteProfile(c echo.Context) error {
	if !adminRequest(c) || !h.authenticated(c) {
		return echo.ErrUnauthorized
	}
	if err := h.store.delete(c.Param("id")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return echo.ErrNotFound
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "No se pudo eliminar el perfil").SetInternal(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) profileStatus(c echo.Context) error {
	profiles, err := h.store.list()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "No se pueden leer los perfiles").SetInternal(err)
	}
	for _, p := range profiles {
		if p.ID != c.Param("id") || (!p.Visible && !h.authenticated(c)) {
			continue
		}
		req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodGet, p.URL+"/api/remote/status", nil)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, "Dirección del sitio inválida").SetInternal(err)
		}
		if p.Password != "" {
			req.SetBasicAuth("", p.Password)
		} else {
			req.Header.Set("Authorization", "Bearer "+p.Token)
		}
		response, err := h.client.Do(req)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, "No se puede conectar al sitio remoto").SetInternal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("El sitio remoto respondió con código %d", response.StatusCode))
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
		if err != nil || len(data) > 2*1024*1024 || !json.Valid(data) {
			return echo.NewHTTPError(http.StatusBadGateway, "Respuesta inválida del sitio remoto")
		}
		if p.Header != nil && (p.Header.Title != "" || p.Header.Subtitle != "" || p.Header.Badge != "") {
			data, err = applyHeaderOverrides(data, *p.Header)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadGateway, "Cabecera inválida del sitio remoto").SetInternal(err)
			}
		}
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, data)
	}
	return echo.ErrNotFound
}

func applyHeaderOverrides(data []byte, overrides ProfileHeader) ([]byte, error) {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(data, &response); err != nil || response == nil {
		return nil, errors.New("respuesta no es un objeto")
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(response["header"], &header); err != nil || header == nil {
		return nil, errors.New("cabecera no es un objeto")
	}
	if overrides.Subtitle != "" {
		var lastUpdate string
		if err := json.Unmarshal(header["lastUpdate"], &lastUpdate); err == nil && lastUpdate != "" {
			overrides.Subtitle += " · última lectura " + lastUpdate
		}
	}
	for key, value := range map[string]string{
		"title": overrides.Title, "subtitle": overrides.Subtitle, "badge": overrides.Badge,
	} {
		if value != "" {
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			header[key] = encoded
		}
	}
	encodedHeader, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	response["header"] = encodedHeader
	return json.Marshal(response)
}
