package api

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmon/bwmon/internal/acl"
	"github.com/bwmon/bwmon/internal/accounting"
	"github.com/bwmon/bwmon/internal/capture"
	"github.com/bwmon/bwmon/internal/config"
	"github.com/bwmon/bwmon/internal/snmp"
)

//go:embed web/static/*
var staticFiles embed.FS

// Services holds runtime dependencies for HTTP handlers.
type Services struct {
	ConfigPath   string
	EnvFiles     []string
	Simulate     bool
	InsecureSNMP bool
	InsecureHTTP bool

	mu      sync.RWMutex
	cfg     *config.Config
	store   *accounting.Store
	snmp    *snmp.Agent
	capture capture.Runner
	httpACL *acl.NetSet
	auth    *HTTPAuth

	OnConfigChange func(*config.Config) error
}

// NewServices wires initial state.
func NewServices(cfg *config.Config, store *accounting.Store, agent *snmp.Agent, cap capture.Runner, path string, simulate bool) (*Services, error) {
	s := &Services{
		ConfigPath: path,
		Simulate:   simulate,
		cfg:        cfg,
		store:      store,
		snmp:       agent,
		capture:    cap,
	}
	if err := s.refreshHTTPACL(); err != nil {
		return nil, err
	}
	if err := s.syncHTTPAuth(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Services) syncHTTPAuth() error {
	users, on, err := config.HTTPUsersFromEnv()
	if err != nil {
		return err
	}
	if !on {
		s.auth = nil
		return nil
	}
	secret := strings.TrimSpace(s.cfg.HTTP.SessionSecret)
	if secret == "" {
		secret, err = config.RandomSessionSecret()
		if err != nil {
			return err
		}
		s.cfg.HTTP.SessionSecret = secret
		if err := s.cfg.Save(s.ConfigPath); err != nil {
			return err
		}
	}
	s.auth = NewHTTPAuth(users, secret)
	return nil
}

func (s *Services) refreshHTTPACL() error {
	if s.cfg.HTTP.InsecureAllowAll {
		s.httpACL = nil
		return nil
	}
	ns, err := acl.Parse(s.cfg.HTTP.Allowed)
	if err != nil {
		return err
	}
	s.httpACL = ns
	return nil
}

// Handler returns the root HTTP handler.
func (s *Services) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(staticFiles, "web/static")
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/api/login", s.withACL(s.handleLogin))
	mux.HandleFunc("/api/logout", s.withACL(s.handleLogout))
	mux.HandleFunc("/api/session", s.withACL(s.handleSession))
	mux.HandleFunc("/api/stats", s.withAPI(s.handleStats))
	mux.HandleFunc("/api/queues", s.withAPI(s.handleQueues))
	mux.HandleFunc("/api/queues/", s.withAPI(s.handleQueueByID))
	mux.HandleFunc("/api/settings", s.withAPI(s.handleSettings))
	mux.HandleFunc("/api/reload-config", s.withAPI(s.handleReload))
	mux.HandleFunc("/api/meta", s.withAPI(s.handleMeta))
	mux.HandleFunc("/api/whoami", s.handleWhoami)
	return mux
}

func clientIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := net.ParseIP(strings.TrimSpace(parts[0])); ip != nil {
			return ip
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		if ip := net.ParseIP(xr); ip != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

func (s *Services) handleWhoami(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	ipStr := ""
	if ip != nil {
		ipStr = ip.String()
	}
	allowed := s.httpACL == nil
	if s.httpACL != nil && ip != nil {
		allowed = s.httpACL.Contains(ip)
	}
	writeJSON(w, map[string]interface{}{
		"clientIp": ipStr,
		"allowed":  allowed,
	})
}

func (s *Services) withACL(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.httpACL != nil {
			ip := clientIP(r)
			if ip == nil || !s.httpACL.Contains(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				ipStr := ""
				if ip != nil {
					ipStr = ip.String()
				}
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":    "forbidden",
					"clientIp": ipStr,
					"hint":     "Add clientIp (or its CIDR) to http.allowed in config.yaml, then reload bwmon",
				})
				return
			}
		}
		next(w, r)
	}
}

func (s *Services) handleMeta(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	writeJSON(w, map[string]interface{}{
		"pen":       cfg.SNMP.PEN,
		"oidBase":   cfg.TableBase(),
		"tableBase": "." + cfg.TableBase(),
		"suffix":    cfg.SNMP.OIDSuffix,
		"simulate":  s.Simulate,
	})
}

func (s *Services) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.cfg
	store := s.store
	s.mu.RUnlock()
	snap := store.Snapshot()
	type row struct {
		Index    int    `json:"index"`
		Target   string `json:"target"`
		Label    string `json:"label"`
		BytesIn  uint64 `json:"bytesIn"`
		BytesOut uint64 `json:"bytesOut"`
		OIDIn    string `json:"oidIn"`
		OIDOut   string `json:"oidOut"`
		OIDName  string `json:"oidName"`
	}
	var out []row
	for _, q := range cfg.Queues {
		p := snap[q.Index]
		out = append(out, row{
			Index:    q.Index,
			Target:   q.Target,
			Label:    q.Label,
			BytesIn:  p.BytesIn,
			BytesOut: p.BytesOut,
			OIDIn:    cfg.OIDFull(cfg.SNMP.OIDSuffix.BytesIn, q.Index),
			OIDOut:   cfg.OIDFull(cfg.SNMP.OIDSuffix.BytesOut, q.Index),
			OIDName:  cfg.OIDFull(cfg.SNMP.OIDSuffix.TargetName, q.Index),
		})
	}
	writeJSON(w, out)
}

func (s *Services) handleQueues(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		cfg := s.cfg
		s.mu.RUnlock()
		writeJSON(w, cfg.Queues)
	case http.MethodPost:
		var body struct {
			Target string `json:"target"`
			Label  string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.ValidateTarget(body.Target); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		idx := s.cfg.AssignNextIndex()
		s.cfg.Queues = append(s.cfg.Queues, config.Queue{Index: idx, Target: strings.TrimSpace(body.Target), Label: strings.TrimSpace(body.Label)})
		if err := s.persistAndReloadLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		q := s.cfg.Queues[len(s.cfg.Queues)-1]
		s.mu.Unlock()
		writeJSON(w, q)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Services) handleQueueByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/queues/")
	idStr = strings.TrimSuffix(idStr, "/")
	idx, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		s.mu.Lock()
		found := false
		for i, q := range s.cfg.Queues {
			if q.Index == idx {
				s.cfg.Queues = append(s.cfg.Queues[:i], s.cfg.Queues[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			s.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := s.persistAndReloadLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPatch:
		var body struct {
			Label string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		found := false
		for i, q := range s.cfg.Queues {
			if q.Index == idx {
				s.cfg.Queues[i].Label = strings.TrimSpace(body.Label)
				found = true
				break
			}
		}
		if !found {
			s.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := s.persistAndReloadLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Services) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		cfg := s.cfg
		s.mu.RUnlock()
		writeJSON(w, map[string]interface{}{
			"capture": cfg.Capture,
			"snmp": map[string]interface{}{
				"listen":      cfg.SNMP.Listen,
				"pen":         cfg.SNMP.PEN,
				"oid_base":    cfg.SNMP.OIDBase,
				"oid_suffix":  cfg.SNMP.OIDSuffix,
				"communities": cfg.SNMP.Communities,
			},
			"http": map[string]interface{}{
				"listen":              cfg.HTTP.Listen,
				"allowed":             cfg.HTTP.Allowed,
				"sessionSecretSet":    strings.TrimSpace(cfg.HTTP.SessionSecret) != "",
			},
		})
	case http.MethodPut:
		var body struct {
			Capture   config.CaptureConfig `json:"capture"`
			SNMP      config.SNMPConfig    `json:"snmp"`
			HTTP      struct {
				Listen        string   `json:"listen"`
				Allowed       []string `json:"allowed"`
				SessionSecret string   `json:"session_secret"`
			} `json:"http"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.cfg.Capture = body.Capture
		if body.SNMP.Listen != "" {
			s.cfg.SNMP.Listen = body.SNMP.Listen
		}
		s.cfg.SNMP.PEN = body.SNMP.PEN
		s.cfg.SNMP.OIDBase = body.SNMP.OIDBase
		if body.SNMP.OIDSuffix.QueueCount != "" {
			s.cfg.SNMP.OIDSuffix = body.SNMP.OIDSuffix
		}
		s.cfg.SNMP.Communities = body.SNMP.Communities
		if body.HTTP.Listen != "" {
			s.cfg.HTTP.Listen = body.HTTP.Listen
		}
		s.cfg.HTTP.Allowed = body.HTTP.Allowed
		if strings.TrimSpace(body.HTTP.SessionSecret) != "" {
			s.cfg.HTTP.SessionSecret = strings.TrimSpace(body.HTTP.SessionSecret)
		}
		if err := s.cfg.Validate(s.InsecureSNMP || s.cfg.SNMP.InsecureAllowAll, s.InsecureHTTP || s.cfg.HTTP.InsecureAllowAll); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.persistAndReloadLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.syncHTTPAuth(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Unlock()
		writeJSON(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Services) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := config.Load(s.ConfigPath, s.InsecureSNMP || s.cfg.SNMP.InsecureAllowAll, s.InsecureHTTP || s.cfg.HTTP.InsecureAllowAll, s.EnvFiles...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.cfg = cfg
	if err := s.persistAndReloadLocked(); err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.syncHTTPAuth(); err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Unlock()
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Services) persistAndReloadLocked() error {
	if err := s.cfg.Save(s.ConfigPath); err != nil {
		return err
	}
	s.store.Rebuild(s.cfg.Queues)
	if err := s.snmp.Reload(s.cfg); err != nil {
		return err
	}
	if err := s.refreshHTTPACL(); err != nil {
		return err
	}
	if s.OnConfigChange != nil {
		return s.OnConfigChange(s.cfg)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// RateTracker helps compute bps between stat polls (optional client-side).
type RateTracker struct {
	last     map[int]counterSample
	lastTime time.Time
}

type counterSample struct {
	in, out uint64
}

func (rt *RateTracker) Rates(now time.Time, snap map[int]accounting.CounterPair) (map[int][2]float64, bool) {
	if rt.last == nil {
		rt.last = make(map[int]counterSample)
		rt.lastTime = now
		for idx, p := range snap {
			rt.last[idx] = counterSample{p.BytesIn, p.BytesOut}
		}
		return nil, false
	}
	dt := now.Sub(rt.lastTime).Seconds()
	if dt <= 0 {
		return nil, false
	}
	out := make(map[int][2]float64)
	for idx, p := range snap {
		prev := rt.last[idx]
		out[idx] = [2]float64{
			float64(p.BytesIn-prev.in) * 8 / dt,
			float64(p.BytesOut-prev.out) * 8 / dt,
		}
		rt.last[idx] = counterSample{p.BytesIn, p.BytesOut}
	}
	rt.lastTime = now
	return out, true
}

// DrainBody closes request bodies.
func DrainBody(r *http.Request) {
	if r.Body != nil {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
}
