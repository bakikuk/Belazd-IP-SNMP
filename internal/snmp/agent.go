package snmp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/bwmon/bwmon/internal/acl"
	"github.com/bwmon/bwmon/internal/accounting"
	"github.com/bwmon/bwmon/internal/config"
	"github.com/gosnmp/gosnmp"
	GoSNMPServer "github.com/slayercat/GoSNMPServer"
)

// Agent serves SNMP with community + source IP ACL.
type Agent struct {
	cfg         *config.Config
	store       *accounting.Store
	master      GoSNMPServer.MasterAgent
	listen      string
	insecure    bool
	communities []communityACL
	conn        *net.UDPConn
	mu          sync.Mutex
}

type communityACL struct {
	name    string
	allowed *acl.NetSet
}

// NewAgent builds an SNMP agent.
func NewAgent(cfg *config.Config, store *accounting.Store) (*Agent, error) {
	a := &Agent{cfg: cfg, store: store, listen: cfg.SNMP.Listen, insecure: cfg.SNMP.InsecureAllowAll}
	if err := a.rebuildMaster(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Agent) rebuildMaster() error {
	a.communities = a.communities[:0]
	var subAgents []*GoSNMPServer.SubAgent
	for _, c := range a.cfg.SNMP.Communities {
		ns, err := acl.Parse(c.Allowed)
		if err != nil && !a.insecure {
			return fmt.Errorf("snmp community %q: %w", c.Community, err)
		}
		a.communities = append(a.communities, communityACL{name: c.Community, allowed: ns})
		subAgents = append(subAgents, &GoSNMPServer.SubAgent{
			CommunityIDs: []string{c.Community},
			OIDs:         a.buildOIDs(),
		})
	}
	if len(subAgents) == 0 {
		subAgents = append(subAgents, &GoSNMPServer.SubAgent{
			CommunityIDs: []string{"public"},
			OIDs:         a.buildOIDs(),
		})
	}
	logger := GoSNMPServer.NewDiscardLogger()
	a.master = GoSNMPServer.MasterAgent{
		Logger:         logger,
		SecurityConfig: GoSNMPServer.SecurityConfig{NoSecurity: true},
		SubAgents:      subAgents,
	}
	return a.master.ReadyForWork()
}

// Reload refreshes after config change.
func (a *Agent) Reload(cfg *config.Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cfg
	a.insecure = cfg.SNMP.InsecureAllowAll
	a.listen = cfg.SNMP.Listen
	return a.rebuildMaster()
}

func (a *Agent) buildOIDs() []*GoSNMPServer.PDUValueControlItem {
	cfg := a.cfg
	store := a.store
	items := []*GoSNMPServer.PDUValueControlItem{
		{
			OID:  trimDot(cfg.OIDFull(cfg.SNMP.OIDSuffix.QueueCount, 0)),
			Type: gosnmp.Integer,
			OnGet: func() (interface{}, error) {
				return len(cfg.Queues), nil
			},
		},
	}
	for _, q := range cfg.Queues {
		idx := q.Index
		display := q.DisplayName()
		nameOID := trimDot(cfg.OIDFull(cfg.SNMP.OIDSuffix.TargetName, idx))
		inOID := trimDot(cfg.OIDFull(cfg.SNMP.OIDSuffix.BytesIn, idx))
		outOID := trimDot(cfg.OIDFull(cfg.SNMP.OIDSuffix.BytesOut, idx))
		items = append(items,
			&GoSNMPServer.PDUValueControlItem{
				OID:  nameOID,
				Type: gosnmp.OctetString,
				OnGet: func() (interface{}, error) {
					a.mu.Lock()
					defer a.mu.Unlock()
					for _, qq := range a.cfg.Queues {
						if qq.Index == idx {
							return qq.DisplayName(), nil
						}
					}
					return display, nil
				},
			},
			&GoSNMPServer.PDUValueControlItem{
				OID:  inOID,
				Type: gosnmp.Counter64,
				OnGet: func() (interface{}, error) {
					if p, ok := store.Snapshot()[idx]; ok {
						return p.BytesIn, nil
					}
					return uint64(0), nil
				},
			},
			&GoSNMPServer.PDUValueControlItem{
				OID:  outOID,
				Type: gosnmp.Counter64,
				OnGet: func() (interface{}, error) {
					if p, ok := store.Snapshot()[idx]; ok {
						return p.BytesOut, nil
					}
					return uint64(0), nil
				},
			},
		)
	}
	return items
}

func trimDot(oid string) string {
	if len(oid) > 0 && oid[0] == '.' {
		return oid[1:]
	}
	return oid
}

// Listen starts the UDP socket.
func (a *Agent) Listen() error {
	addr, err := net.ResolveUDPAddr("udp", a.listen)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	a.conn = conn
	return nil
}

// Close stops listening.
func (a *Agent) Close() {
	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
}

// Serve runs until stop is closed.
func (a *Agent) Serve(stop <-chan struct{}) {
	buf := make([]byte, 65535)
	for {
		select {
		case <-stop:
			return
		default:
		}
		_ = a.conn.SetReadDeadline(deadlineSoon())
		n, remote, err := a.conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-stop:
				return
			default:
				continue
			}
		}
		pkt := append([]byte(nil), buf[:n]...)
		go a.handlePacket(pkt, remote)
	}
}

func (a *Agent) handlePacket(raw []byte, remote *net.UDPAddr) {
	if !a.authorize(raw, remote.IP) {
		return
	}
	out, err := a.master.ResponseForBuffer(raw)
	if err != nil || len(out) == 0 {
		return
	}
	_, _ = a.conn.WriteToUDP(out, remote)
}

func (a *Agent) authorize(raw []byte, ip net.IP) bool {
	if a.insecure {
		return true
	}
	community := parseCommunity(raw)
	if community == "" {
		return false
	}
	for _, c := range a.communities {
		if c.name != community {
			continue
		}
		if c.allowed != nil && c.allowed.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCommunity(raw []byte) string {
	handle := gosnmp.GoSNMP{}
	pkt, err := handle.SnmpDecodePacket(raw)
	if err != nil {
		return ""
	}
	if pkt.Version == gosnmp.Version2c || pkt.Version == gosnmp.Version1 {
		return pkt.Community
	}
	return ""
}

func deadlineSoon() time.Time {
	return time.Now().Add(500 * time.Millisecond)
}
