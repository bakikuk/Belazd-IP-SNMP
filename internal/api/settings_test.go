package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/bakikuk/belazd-ip-snmp/internal/accounting"
	"github.com/bakikuk/belazd-ip-snmp/internal/config"
	"github.com/bakikuk/belazd-ip-snmp/internal/db"
)

func TestSettingsCaptureRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := config.Default()
	cfg.Capture.Interfaces = []string{"ens34", "ens35"}
	cfg.Capture.Interface = "ens34"
	cfg.SNMP.Listen = ":1161"
	cfg.SNMP.Communities = []config.CommunityEntry{{Community: "public", Allowed: []string{"127.0.0.1"}}}
	cfg.HTTP.Allowed = []string{"127.0.0.1/32"}
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	cfg, err = store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	svc, err := NewServices(cfg, accounting.NewStore(cfg.Queues), nil, nil, store, false)
	if err != nil {
		t.Fatal(err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getRec := httptest.NewRecorder()
	svc.handleSettings(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	cap, _ := got["capture"].(map[string]interface{})
	if cap["interface"] != "ens34" {
		t.Fatalf("get capture.interface=%v", cap["interface"])
	}
	ifaces, _ := cap["interfaces"].([]interface{})
	if len(ifaces) < 1 {
		t.Fatalf("get capture.interfaces=%v", cap["interfaces"])
	}

	putBody := []byte(`{"capture":{"interface":"ens35","snaplen":65535}}`)
	var body struct {
		Capture config.CaptureConfig `json:"capture"`
	}
	if err := json.Unmarshal(putBody, &body); err != nil {
		t.Fatal(err)
	}
	if body.Capture.Interface != "ens35" {
		t.Fatalf("put decode interface=%q", body.Capture.Interface)
	}
	cfg.Capture = body.Capture
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Capture.Interface != "ens35" {
		t.Fatalf("sqlite interface=%q", loaded.Capture.Interface)
	}
}
