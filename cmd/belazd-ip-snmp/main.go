package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bakikuk/belazd-ip-snmp/internal/accounting"
	"github.com/bakikuk/belazd-ip-snmp/internal/api"
	"github.com/bakikuk/belazd-ip-snmp/internal/capture"
	"github.com/bakikuk/belazd-ip-snmp/internal/config"
	"github.com/bakikuk/belazd-ip-snmp/internal/db"
	"github.com/bakikuk/belazd-ip-snmp/internal/snmp"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	dataPath := flag.String("data", "", "SQLite database path (default: belazd.sqlite beside binary)")
	_ = flag.String("config", "", "deprecated (ignored): settings live in belazd.sqlite")
	simulate := flag.Bool("simulate", false, "simulate counters without capture")
	insecureSNMP := flag.Bool("insecure-snmp-open", false, "allow SNMP from any IP (lab only)")
	insecureHTTP := flag.Bool("insecure-http-open", false, "allow HTTP from any IP (lab only)")
	flag.Parse()

	dbStore, err := db.Open(*dataPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer dbStore.Close()

	if err := dbStore.Bootstrap(db.BootstrapOptions{
		ConfigYAMLPath: db.ConfigYAMLBesideExe(),
		InsecureSNMP:   *insecureSNMP,
		InsecureHTTP:   *insecureHTTP,
	}); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	cfg, err := dbStore.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if *insecureSNMP {
		cfg.SNMP.InsecureAllowAll = true
	}
	if *insecureHTTP {
		cfg.HTTP.InsecureAllowAll = true
	}
	if err := cfg.Validate(*insecureSNMP, *insecureHTTP); err != nil {
		log.Fatalf("config: %v", err)
	}

	accStore := accounting.NewStore(cfg.Queues)
	agent, err := snmp.NewAgent(cfg, accStore)
	if err != nil {
		log.Fatalf("snmp: %v", err)
	}
	if err := agent.Listen(); err != nil {
		log.Fatalf("snmp listen: %v", err)
	}

	cfg.Capture.NormalizeInterfaces()
	if !*simulate && cfg.HasIPCaptureQueues() {
		for _, name := range cfg.Capture.Interfaces {
			if _, err := net.InterfaceByName(name); err != nil {
				log.Fatalf("%s", capture.FormatInterfaceHint(name))
			}
		}
	}

	var capRunner capture.Runner
	capRunner, err = capture.NewRunner(accStore, cfg, *simulate)
	if err != nil {
		log.Fatalf("capture: %v", err)
	}
	if capRunner != nil {
		if err := capRunner.Start(); err != nil {
			log.Fatalf("capture start: %v\n%s\n\nCaps: setcap cap_net_raw,cap_net_bind_service=+ep ./Belazd-IP-SNMP  or  systemd unit\nLab without capture: -simulate", err, capture.FormatInterfaceHint(cfg.Capture.Interface))
		}
	}

	svc, err := api.NewServices(cfg, accStore, agent, capRunner, dbStore, *simulate)
	if err != nil {
		log.Fatalf("api: %v", err)
	}
	svc.InsecureSNMP = *insecureSNMP
	svc.InsecureHTTP = *insecureHTTP
	svc.OnConfigChange = func(c *config.Config) error {
		var err error
		capRunner, err = capture.Restart(capRunner, accStore, c, *simulate)
		return err
	}

	stopSNMP := make(chan struct{})
	go agent.Serve(stopSNMP)

	httpAddr := config.FixedHTTPListen
	srv := &http.Server{Addr: httpAddr, Handler: svc.Handler()}
	go func() {
		log.Printf("Belazd-IP-SNMP %s (%s) db=%s http=%s snmp=%s simulate=%v", version, commit, dbStore.Path(), httpAddr, cfg.SNMP.Listen, *simulate)
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
