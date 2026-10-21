package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bakikuk/beladz-ip-snmp/internal/config"
)

const (
	sessionCookie   = "beladz_session"
	sessionLifetime = 24 * time.Hour
)

// HTTPAuth enables login when BELADZ_HTTP_USER / BELADZ_HTTP_PASSWORD are set in .env.
type HTTPAuth struct {
	users  map[string]string // username -> password
	secret []byte
}

func NewHTTPAuth(users []config.HTTPUser, sessionSecret string) *HTTPAuth {
	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.Username] = u.Password
	}
	return &HTTPAuth{users: m, secret: []byte(sessionSecret)}
}

func (a *HTTPAuth) Enabled() bool {
	return a != nil && len(a.users) > 0
}

func (a *HTTPAuth) checkCredentials(user, pass string) bool {
	want, ok := a.users[user]
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pass), []byte(want)) == 1
}

func (a *HTTPAuth) issueToken(user string) (string, error) {
	exp := time.Now().Add(sessionLifetime).Unix()
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
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return a.validateToken(c.Value)
	}
	return "", false
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
	out := map[string]interface{}{
		"authRequired": s.auth != nil && s.auth.Enabled(),
		"clientIp":     ipStr,
		"aclAllowed":   aclOK,
	}
	if s.auth != nil && s.auth.Enabled() {
		if user, ok := s.auth.userFromRequest(r); ok {
			out["loggedIn"] = true
			out["user"] = user
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
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionLifetime.Seconds()),
	})
	writeJSON(w, map[string]string{"status": "ok", "user": user})
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
		if _, ok := s.auth.userFromRequest(r); !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":         "unauthorized",
				"loginRequired": true,
				"hint":          "POST /api/login with BELADZ_HTTP_USER / BELADZ_HTTP_PASSWORD from .env",
			})
			return
		}
		next(w, r)
	}
}

func (s *Services) withAPI(next http.HandlerFunc) http.HandlerFunc {
	return s.withACL(s.withLogin(next))
}
