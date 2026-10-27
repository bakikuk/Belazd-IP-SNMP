package api

import (
	"crypto/rand"
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

	"github.com/bakikuk/belazd-ip-snmp/internal/acl"
	"github.com/bakikuk/belazd-ip-snmp/internal/accounting"
	"github.com/bakikuk/belazd-ip-snmp/internal/capture"
	"github.com/bakikuk/belazd-ip-snmp/internal/config"
	"github.com/bakikuk/belazd-ip-snmp/internal/db"
	"github.com/bakikuk/belazd-ip-snmp/internal/snmp"
)

//go:embed web/static/*
var staticFiles embed.FS

// Services holds runtime dependencies for HTTP handlers.
type Services struct {
	Simulate     bool
	InsecureSNMP bool
	InsecureHTTP bool

	mu      sync.RWMutex
	cfg     *config.Config
	db      *db.Store
	store   *accounting.Store
	snmp    *snmp.Agent
	capture capture.Runner
	httpACL *acl.NetSet
	auth       *HTTPAuth
	authBootID []byte

	statsRatesMu sync.Mutex
	statsRates   RateTracker

	OnConfigChange func(*config.Config) error
}

// NewServices wires initial state.
func NewServices(cfg *config.Config, store *accounting.Store, agent *snmp.Agent, cap capture.Runner, database *db.Store, simulate bool) (*Services, error) {
	bootID := make([]byte, 16)
	if _, err := rand.Read(bootID); err != nil {
		return nil, err
	}
	s := &Services{
		Simulate:   simulate,
		cfg:        cfg,
		db:         database,
		store:      store,
		snmp:       agent,
		capture:    cap,
		authBootID: bootID,
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
	if len(s.cfg.Users) == 0 {
		s.auth = nil
		return nil
	}
	secret := strings.TrimSpace(s.cfg.HTTP.SessionSecret)
	if secret == "" {
		var err error
		secret, err = config.RandomSessionSecret()
		if err != nil {
			return err
		}
		s.cfg.HTTP.SessionSecret = secret
		if err := s.db.SaveConfig(s.cfg); err != nil {
			return err
		}
	}
	s.auth = NewHTTPAuth(s.cfg, secret, s.authBootID)
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
	mux.HandleFunc("/api/user-groups", s.withAPI(s.handleUserGroups))
	mux.HandleFunc("/api/user-groups/", s.withAPI(s.handleUserGroupByID))
	mux.HandleFunc("/api/users", s.withAPI(s.handleUsers))
	mux.HandleFunc("/api/users/", s.withAPI(s.handleUserByName))
	mux.HandleFunc("/api/me/password", s.withACL(s.withLogin(s.handleMePassword)))
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
					"hint":     "Add clientIp (or its CIDR) to http.allowed in Settings, then reload",
				})
				return
			}
		}
		next(w, r)
	}
}

func (s *Services) handleMeta(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePerm(w, r, config.ModuleQueues, config.PermRead); !ok {
		return
	}
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	cfg.Capture.NormalizeInterfaces()
	writeJSON(w, map[string]interface{}{
		"pen":       cfg.SNMP.PEN,
		"oidBase":   cfg.TableBase(),
		"tableBase": "." + cfg.TableBase(),
		"suffix":    cfg.SNMP.OIDSuffix,
		"simulate":  s.Simulate,
		"capture": map[string]interface{}{
			"interface":  cfg.Capture.DefaultInterface(),
			"interfaces": cfg.Capture.Interfaces,
			"default":    cfg.Capture.DefaultInterface(),
			"snaplen":    cfg.Capture.Snaplen,
		},
		"captureLive": s.capture != nil && !s.Simulate,
	})
}

func (s *Services) handleStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePerm(w, r, config.ModuleQueues, config.PermRead); !ok {
		return
	}
	s.mu.RLock()
	cfg := s.cfg
	store := s.store
	s.mu.RUnlock()
	snap := store.Snapshot()
	now := time.Now()
	s.statsRatesMu.Lock()
	rates, ratesReady := s.statsRates.Rates(now, snap)
	s.statsRatesMu.Unlock()
	type row struct {
		Index             int     `json:"index"`
		Target            string  `json:"target"`
		TargetKind        string  `json:"targetKind"`
		Label             string  `json:"label"`
		CaptureIface      string  `json:"captureIface,omitempty"`
		CaptureIfaceError string  `json:"captureIfaceError,omitempty"`
		BytesIn           uint64  `json:"bytesIn"`
		BytesOut          uint64  `json:"bytesOut"`
		BytesInPerSec     float64 `json:"bytesInPerSec,omitempty"`
		BytesOutPerSec    float64 `json:"bytesOutPerSec,omitempty"`
		RatesReady        bool    `json:"ratesReady"`
		OIDIn             string  `json:"oidIn"`
		OIDOut            string  `json:"oidOut"`
		OIDName           string  `json:"oidName"`
	}
	var out []row
	for _, q := range cfg.Queues {
		p := snap[q.Index]
		captureIface, captureErr := cfg.ResolveQueueCaptureIface(q)
		rowRatesReady := ratesReady && captureErr == ""
		r := row{
			Index:             q.Index,
			Target:            q.Target,
			TargetKind:        config.TargetKind(q.Target),
			Label:             q.Label,
			CaptureIface:      captureIface,
			CaptureIfaceError: captureErr,
			BytesIn:           p.BytesIn,
			BytesOut:          p.BytesOut,
			RatesReady:        rowRatesReady,
			OIDIn:             cfg.OIDFull(cfg.SNMP.OIDSuffix.BytesIn, q.Index),
			OIDOut:            cfg.OIDFull(cfg.SNMP.OIDSuffix.BytesOut, q.Index),
			OIDName:           cfg.OIDFull(cfg.SNMP.OIDSuffix.TargetName, q.Index),
		}
		if rowRatesReady {
			if pair, ok := rates[q.Index]; ok {
				r.BytesInPerSec = pair[0]
				r.BytesOutPerSec = pair[1]
			}
		}
		out = append(out, r)
	}
	writeJSON(w, out)
}

func (s *Services) handleQueues(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requirePerm(w, r, config.ModuleQueues, config.PermRead); !ok {
			return
		}
		s.mu.RLock()
		cfg := s.cfg
		s.mu.RUnlock()
		writeJSON(w, cfg.Queues)
	case http.MethodPost:
		if _, ok := s.requirePerm(w, r, config.ModuleQueues, config.PermWrite); !ok {
			return
		}
		var body struct {
			Target       string `json:"target"`
			Label        string `json:"label"`
			CaptureIface string `json:"capture_iface"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.ValidateTarget(body.Target); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		q := config.Queue{
			Index:        0,
			Target:       strings.TrimSpace(body.Target),
			Label:        strings.TrimSpace(body.Label),
			CaptureIface: strings.TrimSpace(body.CaptureIface),
		}
		if config.IsIfaceTarget(q.Target) {
			q.CaptureIface = ""
		}
		s.mu.Lock()
		if err := s.cfg.ValidateQueueCaptureIface(q); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		idx := s.cfg.AssignNextIndex()
		q.Index = idx
		s.cfg.Queues = append(s.cfg.Queues, q)
		if err := s.persistAndReloadLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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
		if _, ok := s.requirePerm(w, r, config.ModuleQueues, config.PermDelete); !ok {
			return
		}
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
		if _, ok := s.requirePerm(w, r, config.ModuleQueues, config.PermWrite); !ok {
			return
		}
		var body struct {
			Label        string `json:"label"`
			CaptureIface string `json:"capture_iface"`
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
				if !config.IsIfaceTarget(q.Target) && config.IsIPTarget(q.Target) {
					s.cfg.Queues[i].CaptureIface = strings.TrimSpace(body.CaptureIface)
					if err := s.cfg.ValidateQueueCaptureIface(s.cfg.Queues[i]); err != nil {
						s.mu.Unlock()
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
				}
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
		if _, ok := s.requirePerm(w, r, config.ModuleSettings, config.PermRead); !ok {
			return
		}
		s.mu.RLock()
		cfg := s.cfg
		s.mu.RUnlock()
		cfg.Capture.NormalizeInterfaces()
		writeJSON(w, map[string]interface{}{
			"capture": map[string]interface{}{
				"interface":  cfg.Capture.DefaultInterface(),
				"interfaces": cfg.Capture.Interfaces,
				"default":    cfg.Capture.DefaultInterface(),
				"snaplen":    cfg.Capture.Snaplen,
			},
			"snmp": map[string]interface{}{
				"listen":      cfg.SNMP.Listen,
				"pen":         cfg.SNMP.PEN,
				"oid_base":    cfg.SNMP.OIDBase,
				"oid_suffix":  cfg.SNMP.OIDSuffix,
				"communities": cfg.SNMP.Communities,
			},
			"http": map[string]interface{}{
				"listen":              config.FixedHTTPListen,
				"listenFixed":         true,
				"allowed":             cfg.HTTP.Allowed,
				"sessionSecretSet":    strings.TrimSpace(cfg.HTTP.SessionSecret) != "",
			},
		})
	case http.MethodPut:
		if _, ok := s.requirePerm(w, r, config.ModuleSettings, config.PermWrite); !ok {
			return
		}
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
		s.cfg.Capture.NormalizeInterfaces()
		if body.SNMP.Listen != "" {
			s.cfg.SNMP.Listen = body.SNMP.Listen
		}
		s.cfg.SNMP.PEN = body.SNMP.PEN
		s.cfg.SNMP.OIDBase = body.SNMP.OIDBase
		s.cfg.SNMP.OIDSuffix.MergeNonEmpty(body.SNMP.OIDSuffix)
		s.cfg.SNMP.OIDSuffix.FillDefaultsIfEmpty()
		s.cfg.SNMP.Communities = body.SNMP.Communities
		s.cfg.HTTP.Listen = config.FixedHTTPListen
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
	if _, ok := s.requirePerm(w, r, config.ModuleSettings, config.PermWrite); !ok {
		return
	}
	cfg, err := s.db.LoadConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.InsecureSNMP {
		cfg.SNMP.InsecureAllowAll = true
	}
	if s.InsecureHTTP {
		cfg.HTTP.InsecureAllowAll = true
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
	if err := s.db.SaveConfig(s.cfg); err != nil {
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

func counterDelta(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	return cur
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
	if dt < 0.2 {
		return nil, false
	}
	out := make(map[int][2]float64)
	for idx, p := range snap {
		prev := rt.last[idx]
		dIn := counterDelta(p.BytesIn, prev.in)
		dOut := counterDelta(p.BytesOut, prev.out)
		out[idx] = [2]float64{
			float64(dIn) / dt,
			float64(dOut) / dt,
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
