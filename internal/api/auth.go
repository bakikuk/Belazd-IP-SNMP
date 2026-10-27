package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

const (
	sessionCookie      = "belazd_session"
	sessionIdleTimeout = time.Hour
)

type ctxKey int

const ctxUserKey ctxKey = 1

type userAuthRecord struct {
	passwordHash string
	groupID      string
}

// HTTPAuth enables login when config has users.
type HTTPAuth struct {
	users  map[string]userAuthRecord
	groups map[string]config.ModulePermissions
	secret []byte
}

func NewHTTPAuth(cfg *config.Config, sessionSecret string, bootID []byte) *HTTPAuth {
	key := []byte(sessionSecret)
	if len(bootID) > 0 {
		key = append(key, bootID...)
	}
	a := &HTTPAuth{
		users:  make(map[string]userAuthRecord, len(cfg.Users)),
		groups: make(map[string]config.ModulePermissions, len(cfg.UserGroups)),
		secret: key,
	}
	for _, g := range cfg.UserGroups {
		a.groups[g.ID] = g.Permissions
	}
	for _, u := range cfg.Users {
		a.users[u.Username] = userAuthRecord{
			passwordHash: u.PasswordHash,
			groupID:      u.GroupID,
		}
	}
	return a
}

func (a *HTTPAuth) Enabled() bool {
	return a != nil && len(a.users) > 0
}

func (a *HTTPAuth) checkCredentials(user, pass string) bool {
	rec, ok := a.users[user]
	if !ok {
		return false
	}
	return config.CheckPassword(rec.passwordHash, pass)
}

func (a *HTTPAuth) groupID(user string) string {
	if rec, ok := a.users[user]; ok {
		return rec.groupID
	}
	return ""
}

func (a *HTTPAuth) permissionsFor(user string) config.ModulePermissions {
	gid := a.groupID(user)
	if gid == "" {
		return config.ModulePermissions{}
	}
	if p, ok := a.groups[gid]; ok {
		return p
	}
	return config.ModulePermissions{}
}

func (a *HTTPAuth) issueToken(user string) (string, error) {
	exp := time.Now().Add(sessionIdleTimeout).Unix()
	payload := user + "|" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (a *HTTPAuth) validateToken(token string) (user string, ok bool) {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	payload := string(payloadBytes)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	if !hmac.Equal(mac.Sum(nil), sigBytes) {
		return "", false
	}
	sep := strings.LastIndex(payload, "|")
	if sep < 0 {
		return "", false
	}
	user = payload[:sep]
	expStr := payload[sep+1:]
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	if _, exists := a.users[user]; !exists {
		return "", false
	}
	return user, true
}

func (a *HTTPAuth) userFromRequest(r *http.Request) (string, bool) {
	if u, ok := r.Context().Value(ctxUserKey).(string); ok && u != "" {
		return u, true
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return a.validateToken(c.Value)
	}
	return "", false
}

func (s *Services) sessionUser(r *http.Request) (string, bool) {
	if s.auth == nil || !s.auth.Enabled() {
		return "", false
	}
	return s.auth.userFromRequest(r)
}

func (s *Services) writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
}

func (s *Services) requirePerm(w http.ResponseWriter, r *http.Request, mod config.PermissionModule, min config.PermissionLevel) (user string, ok bool) {
	if s.auth == nil || !s.auth.Enabled() {
		return "", true
	}
	user, loggedIn := s.auth.userFromRequest(r)
	if !loggedIn {
		return "", false
	}
	if !s.auth.permissionsFor(user).AtLeast(mod, min) {
		s.writeForbidden(w)
		return user, false
	}
	return user, true
}

func (s *Services) withPerm(mod config.PermissionModule, min config.PermissionLevel, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.requirePerm(w, r, mod, min); !ok {
			return
		}
		next(w, r)
	}
}

func (s *Services) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionIdleTimeout.Seconds()),
	})
}

func (s *Services) extendSession(w http.ResponseWriter, user string) {
	if s.auth == nil || !s.auth.Enabled() || user == "" {
		return
	}
	token, err := s.auth.issueToken(user)
	if err != nil {
		return
	}
	s.setSessionCookie(w, token)
}

func (s *Services) handleSession(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	ipStr := ""
	if ip != nil {
		ipStr = ip.String()
	}
	aclOK := s.httpACL == nil
	if s.httpACL != nil && ip != nil {
		aclOK = s.httpACL.Contains(ip)
	}
	s.mu.RLock()
	userCount := len(s.cfg.Users)
	s.mu.RUnlock()
	out := map[string]interface{}{
		"authRequired":       s.auth != nil && s.auth.Enabled(),
		"clientIp":           ipStr,
		"aclAllowed":         aclOK,
		"userCount":          userCount,
		"idleTimeoutSeconds": int(sessionIdleTimeout.Seconds()),
	}
	if s.auth != nil && s.auth.Enabled() {
		if user, ok := s.auth.userFromRequest(r); ok {
			s.extendSession(w, user)
			out["loggedIn"] = true
			out["user"] = user
			out["groupId"] = s.auth.groupID(user)
			out["permissions"] = s.auth.permissionsFor(user).PermissionsJSON()
		} else {
			out["loggedIn"] = false
		}
	} else {
		out["loggedIn"] = true
	}
	writeJSON(w, out)
}

func (s *Services) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil || !s.auth.Enabled() {
		writeJSON(w, map[string]string{"status": "ok", "user": ""})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	user := strings.TrimSpace(body.Username)
	if !s.auth.checkCredentials(user, body.Password) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}
	token, err := s.auth.issueToken(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, token)
	perms := s.auth.permissionsFor(user).PermissionsJSON()
	writeJSON(w, map[string]interface{}{
		"status":      "ok",
		"user":        user,
		"groupId":     s.auth.groupID(user),
		"permissions": perms,
	})
}

func (s *Services) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Services) withLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil || !s.auth.Enabled() {
			next(w, r)
			return
		}
		user, ok := s.auth.userFromRequest(r)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":         "unauthorized",
				"loginRequired": true,
				"hint":          "POST /api/login with your administrator account",
			})
			return
		}
		s.extendSession(w, user)
		r = r.WithContext(context.WithValue(r.Context(), ctxUserKey, user))
		next(w, r)
	}
}

func (s *Services) withAPI(next http.HandlerFunc) http.HandlerFunc {
	return s.withACL(s.withLogin(next))
}
