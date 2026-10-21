package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmon/bwmon/internal/accounting"
	"github.com/bwmon/bwmon/internal/api"
	"github.com/bwmon/bwmon/internal/capture"
	"github.com/bwmon/bwmon/internal/config"
	"github.com/bwmon/bwmon/internal/snmp"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	envFile := flag.String("env", ".env", "dotenv file for secrets (empty to skip)")
	simulate := flag.Bool("simulate", false, "simulate counters without capture")
	insecureSNMP := flag.Bool("insecure-snmp-open", false, "allow SNMP from any IP (lab only)")
	insecureHTTP := flag.Bool("insecure-http-open", false, "allow HTTP from any IP (lab only)")
	flag.Parse()

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		ex := config.Default()
		ex.SNMP.Communities = []config.CommunityEntry{{Community: "public", Allowed: []string{"127.0.0.1"}}}
		ex.HTTP.Allowed = []string{"127.0.0.1"}
		if err := ex.Save(*configPath); err != nil {
			log.Fatalf("create config: %v", err)
		}
		log.Printf("created sample %s — edit communities and http.allowed before production", *configPath)
	}

	var envPaths []string
	if strings.TrimSpace(*envFile) != "" {
		envPaths = []string{*envFile}
	}
	cfg, err := config.Load(*configPath, *insecureSNMP, *insecureHTTP, envPaths...)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if _, _, _, err := config.LoadHTTPAuthFromEnv(); err != nil {
		log.Fatalf("http auth: %v", err)
	}

	store := accounting.NewStore(cfg.Queues)
	agent, err := snmp.NewAgent(cfg, store)
	if err != nil {
		log.Fatalf("snmp: %v", err)
	}
	if err := agent.Listen(); err != nil {
		log.Fatalf("snmp listen: %v", err)
	}

	if !*simulate && cfg.Capture.Interface != "" {
		if _, err := net.InterfaceByName(cfg.Capture.Interface); err != nil {
			log.Fatalf("%s", capture.FormatInterfaceHint(cfg.Capture.Interface))
		}
	}

	var capRunner capture.Runner
	capRunner, err = capture.NewRunner(store, cfg, *simulate)
	if err != nil {
		log.Fatalf("capture: %v", err)
	}
	if capRunner != nil {
		if err := capRunner.Start(); err != nil {
			log.Fatalf("capture start: %v\n%s\n\nCaps: setcap cap_net_raw,cap_net_bind_service=+ep ./bwmon  or  systemd unit\nLab without capture: -simulate", err, capture.FormatInterfaceHint(cfg.Capture.Interface))
		}
	}

	svc, err := api.NewServices(cfg, store, agent, capRunner, *configPath, *simulate)
	if err != nil {
		log.Fatalf("api: %v", err)
	}
	svc.EnvFiles = envPaths
	svc.InsecureSNMP = *insecureSNMP
	svc.InsecureHTTP = *insecureHTTP
	svc.OnConfigChange = func(c *config.Config) error {
		var err error
		capRunner, err = capture.Restart(capRunner, store, c, *simulate)
		return err
	}

	stopSNMP := make(chan struct{})
	go agent.Serve(stopSNMP)

	srv := &http.Server{Addr: cfg.HTTP.Listen, Handler: svc.Handler()}
	go func() {
		log.Printf("bwmon %s (%s) http=%s snmp=%s simulate=%v", version, commit, cfg.HTTP.Listen, cfg.SNMP.Listen, *simulate)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	close(stopSNMP)
	agent.Close()
	if capRunner != nil {
		capRunner.Stop()
	}
	_ = srv.Close()
	fmt.Println("shutdown")
}
