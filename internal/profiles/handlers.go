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
	store           *Store
	client          *http.Client
	cookiePath      string
	mu              sync.Mutex
	mainSettings    MainSettings
	applySettings   func(MainSettings)
	sessions        map[string]session
	login           *rate.Limiter
	remoteFailures  *rate.Limiter
	executeEchoLink func(context.Context, EchoLinkAction) (string, error)
}

type EchoLinkAction struct {
	Action    string `json:"action"`
	Callsign  string `json:"callsign"`
	IP        string `json:"ip"`
	Message   string `json:"message"`
	Permanent bool   `json:"permanent"`
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

func (h *Handler) SetMainSettings(settings MainSettings, apply func(MainSettings)) {
	h.mu.Lock()
	h.mainSettings = settings
	h.applySettings = apply
	h.mu.Unlock()
}

func (h *Handler) adminSettings(c echo.Context) error {
	if !h.authenticated(c) {
		return echo.ErrUnauthorized
	}
	h.mu.Lock()
	settings := h.mainSettings
	configured := h.applySettings != nil
	h.mu.Unlock()
	if !configured {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "La configuración principal no está disponible")
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, settings)
}

func (h *Handler) updateAdminSettings(c echo.Context) error {
	if !adminRequest(c) || !h.authenticated(c) {
		return echo.ErrUnauthorized
	}
	var settings MainSettings
	if err := decodeBody(c, &settings); err != nil {
		return echo.ErrBadRequest
	}
	h.mu.Lock()
	apply := h.applySettings
	if apply == nil {
		h.mu.Unlock()
		return echo.NewHTTPError(http.StatusServiceUnavailable, "La configuración principal no está disponible")
	}
	saved, err := h.store.SaveMainSettings(settings)
	if errors.Is(err, errInvalidMainSettings) {
		h.mu.Unlock()
		return echo.NewHTTPError(http.StatusBadRequest, "El título es obligatorio; el título admite hasta 200 caracteres y el subtítulo hasta 500")
	}
	if err != nil {
		h.mu.Unlock()
		return echo.NewHTTPError(http.StatusInternalServerError, "No se pudo guardar la configuración principal").SetInternal(err)
	}
	h.mainSettings = saved
	apply(saved)
	h.mu.Unlock()
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, saved)
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

func (h *Handler) SetEchoLinkActionExecutor(executor func(context.Context, EchoLinkAction) (string, error)) {
	h.executeEchoLink = executor
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
	group.POST("/api/admin/echolink/action", h.executeEchoLinkAction)
	group.POST("/api/profiles/:id/echolink/action", h.profileEchoLinkAction)
	group.GET("/api/admin/profiles", h.adminProfiles)
	group.GET("/api/admin/settings", h.adminSettings)
	group.PUT("/api/admin/settings", h.updateAdminSettings)
	group.POST("/api/admin/profiles", h.createProfile)
	group.PUT("/api/admin/profiles/:id", h.updateProfile)
	group.DELETE("/api/admin/profiles/:id", h.deleteProfile)
	group.GET("/api/remote/status", func(c echo.Context) error {
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
	scheme := "http"
	if secureRequest(r) {
		scheme = "https"
	}
	return err == nil && origin.Scheme == scheme &&
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
		Secure: secureRequest(c.Request()), SameSite: http.SameSiteStrictMode})
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
	c.SetCookie(&http.Cookie{Name: "ameriboard_admin", Path: h.cookiePath, HttpOnly: true, Secure: secureRequest(c.Request()),
		SameSite: http.SameSiteStrictMode, MaxAge: -1})
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) executeEchoLinkAction(c echo.Context) error {
	if !h.adminActionAuthorized(c) {
		c.Logger().Warn("EchoLink local: solicitud rechazada por falta de autorización")
		return echo.ErrUnauthorized
	}
	if h.executeEchoLink == nil {
		c.Logger().Error("EchoLink local: ejecutor no configurado")
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Las acciones de EchoLink no están disponibles")
	}
	var request EchoLinkAction
	if err := decodeBody(c, &request); err != nil {
		c.Logger().Warnf("EchoLink local: solicitud inválida (tipo de error %T)", err)
		return echo.ErrBadRequest
	}
	action := echoLinkLogAction(request.Action)
	started := time.Now()
	c.Logger().Infof("EchoLink local: acción=%q iniciada", action)
	output, err := h.executeEchoLink(c.Request().Context(), request)
	if err != nil {
		c.Logger().Errorf("EchoLink local: acción=%q falló duración=%s error=%q", action, time.Since(started).Round(time.Millisecond), err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error()).SetInternal(err)
	}
	c.Logger().Infof("EchoLink local: acción=%q proceso finalizado duración=%s", action, time.Since(started).Round(time.Millisecond))
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, map[string]string{"output": output})
}

func echoLinkLogAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "connect", "disconnect", "ban", "ban-list", "unban", "mute", "unmute", "message", "retry-list", "retry-cancel":
		return strings.ToLower(strings.TrimSpace(action))
	default:
		return "inválida"
	}
}

func (h *Handler) adminActionAuthorized(c echo.Context) bool {
	if adminRequest(c) && h.authenticated(c) {
		return true
	}
	_, password, ok := c.Request().BasicAuth()
	if ok {
		hash, err := h.store.credential("admin")
		if err != nil || len(password) > 72 {
			return false
		}
		return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	}
	return false
}

func (h *Handler) profileEchoLinkAction(c echo.Context) error {
	if !adminRequest(c) || !h.authenticated(c) {
		c.Logger().Warn("EchoLink remoto: solicitud rechazada por falta de autorización")
		return echo.ErrUnauthorized
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 16*1024))
	if err != nil {
		c.Logger().Warnf("EchoLink remoto: no se pudo leer la solicitud: %v", err)
		return echo.ErrBadRequest
	}
	var action EchoLinkAction
	if err := json.Unmarshal(body, &action); err != nil {
		c.Logger().Warnf("EchoLink remoto: solicitud inválida (tipo de error %T)", err)
		return echo.ErrBadRequest
	}
	logAction := echoLinkLogAction(action.Action)
	profiles, err := h.store.list()
	if err != nil {
		c.Logger().Errorf("EchoLink remoto: no se pueden leer perfiles: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "No se pueden leer los perfiles").SetInternal(err)
	}
	for _, profile := range profiles {
		if profile.ID != c.Param("id") {
			continue
		}
		started := time.Now()
		c.Logger().Infof("EchoLink remoto: perfil=%q acción=%q iniciada", profile.ID, logAction)
		if profile.Password == "" {
			c.Logger().Warnf("EchoLink remoto: perfil=%q acción=%q sin contraseña de administrador", profile.ID, logAction)
			return echo.NewHTTPError(http.StatusBadGateway, "El perfil remoto requiere una contraseña de administrador")
		}
		req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodPost, profile.URL+"/api/admin/echolink/action", strings.NewReader(string(body)))
		if err != nil {
			c.Logger().Errorf("EchoLink remoto: perfil=%q acción=%q dirección inválida", profile.ID, logAction)
			return echo.NewHTTPError(http.StatusBadGateway, "Dirección del sitio inválida").SetInternal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", c.Request().Host)
		req.Header.Set("X-Ameriboard-Request", "1")
		req.SetBasicAuth("", profile.Password)
		response, err := h.client.Do(req)
		if err != nil {
			c.Logger().Errorf("EchoLink remoto: perfil=%q acción=%q fallo de conexión duración=%s error=%q", profile.ID, logAction, time.Since(started).Round(time.Millisecond), err)
			return echo.NewHTTPError(http.StatusBadGateway, "No se puede conectar al sitio remoto").SetInternal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			c.Logger().Errorf("EchoLink remoto: perfil=%q acción=%q HTTP=%d duración=%s", profile.ID, logAction, response.StatusCode, time.Since(started).Round(time.Millisecond))
			return echo.NewHTTPError(http.StatusBadGateway, "El sitio remoto no pudo ejecutar la acción").SetInternal(fmt.Errorf("estado HTTP %d", response.StatusCode))
		}
		c.Logger().Infof("EchoLink remoto: perfil=%q acción=%q HTTP=%d duración=%s", profile.ID, logAction, response.StatusCode, time.Since(started).Round(time.Millisecond))
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().WriteHeader(http.StatusOK)
		_, err = io.Copy(c.Response(), response.Body)
		if err != nil {
			c.Logger().Errorf("EchoLink remoto: perfil=%q acción=%q error al transmitir respuesta: %v", profile.ID, logAction, err)
		}
		return err
	}
	c.Logger().Warnf("EchoLink remoto: perfil no encontrado acción=%q", logAction)
	return echo.ErrNotFound
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
		if p.Header != nil && (p.Header.Title != "" || p.Header.Subtitle != "" ||
			p.Header.Badge != "" || p.Header.DVSwitchTitle != "" || p.Header.NXDNTitle != "") {
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
	if overrides.DVSwitchTitle != "" {
		encoded, err := json.Marshal(overrides.DVSwitchTitle)
		if err != nil {
			return nil, err
		}
		response["dvSwitchTitle"] = encoded
	}
	if overrides.NXDNTitle != "" {
		encoded, err := json.Marshal(overrides.NXDNTitle)
		if err != nil {
			return nil, err
		}
		response["nxdnTitle"] = encoded
	}
	encodedHeader, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	response["header"] = encodedHeader
	return json.Marshal(response)
}
