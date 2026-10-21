package accounting

import (
	"net"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

// Matcher maps packets to a queue index.
type Matcher struct {
	Index  int
	Target string
	Spec   config.TargetSpec
}

// Store holds counters and matchers.
type Store struct {
	mu        sync.RWMutex
	matchers  []Matcher
	bytesIn   map[int]*atomic.Uint64
	bytesOut  map[int]*atomic.Uint64
	queueMeta map[int]config.Queue
}

func NewStore(queues []config.Queue) *Store {
	s := &Store{
		bytesIn:   make(map[int]*atomic.Uint64),
		bytesOut:  make(map[int]*atomic.Uint64),
		queueMeta: make(map[int]config.Queue),
	}
	s.Rebuild(queues)
	return s
}

// Rebuild replaces matchers and ensures counters exist for each queue index.
func (s *Store) Rebuild(queues []config.Queue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchers = s.matchers[:0]
	for _, q := range queues {
		if config.IsIfaceTarget(q.Target) {
			s.queueMeta[q.Index] = q
			if s.bytesIn[q.Index] == nil {
				s.bytesIn[q.Index] = &atomic.Uint64{}
				s.bytesOut[q.Index] = &atomic.Uint64{}
			}
			continue
		}
		spec, err := config.ParseTarget(q.Target)
		if err != nil {
			continue
		}
		s.matchers = append(s.matchers, Matcher{Index: q.Index, Target: q.Target, Spec: spec})
		s.queueMeta[q.Index] = q
		if s.bytesIn[q.Index] == nil {
			s.bytesIn[q.Index] = &atomic.Uint64{}
			s.bytesOut[q.Index] = &atomic.Uint64{}
		}
	}
	sort.Slice(s.matchers, func(i, j int) bool { return s.matchers[i].Index < s.matchers[j].Index })
}

// AccountIPv4 adds length bytes for src/dst (IPv4).
func (s *Store) AccountIPv4(src, dst net.IP, length uint64) {
	src = src.To4()
	dst = dst.To4()
	if src == nil || dst == nil {
		return
	}
	s.mu.RLock()
	matchers := s.matchers
	s.mu.RUnlock()
	for _, m := range matchers {
		if m.Spec.MatchesIn(dst) {
			s.counterIn(m.Index).Add(length)
		}
		if m.Spec.MatchesOut(src) {
			s.counterOut(m.Index).Add(length)
		}
	}
}

func (s *Store) counterIn(idx int) *atomic.Uint64 {
	s.mu.RLock()
	c := s.bytesIn[idx]
	s.mu.RUnlock()
	if c == nil {
		s.mu.Lock()
		if s.bytesIn[idx] == nil {
			s.bytesIn[idx] = &atomic.Uint64{}
		}
		c = s.bytesIn[idx]
		s.mu.Unlock()
	}
	return c
}

func (s *Store) counterOut(idx int) *atomic.Uint64 {
	s.mu.RLock()
	c := s.bytesOut[idx]
	s.mu.RUnlock()
	if c == nil {
		s.mu.Lock()
		if s.bytesOut[idx] == nil {
			s.bytesOut[idx] = &atomic.Uint64{}
		}
		c = s.bytesOut[idx]
		s.mu.Unlock()
	}
	return c
}

// Snapshot returns current counters per queue index.
func (s *Store) Snapshot() map[int]CounterPair {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[int]CounterPair, len(s.queueMeta))
	for idx, q := range s.queueMeta {
		var inBytes, outBytes uint64
		if c := s.bytesIn[idx]; c != nil {
			inBytes = c.Load()
		}
		if c := s.bytesOut[idx]; c != nil {
			outBytes = c.Load()
		}
		result[idx] = CounterPair{Queue: q, BytesIn: inBytes, BytesOut: outBytes}
	}
	return result
}

type CounterPair struct {
	Queue    config.Queue
	BytesIn  uint64
	BytesOut uint64
}

// SimulateAdd adds fake traffic for testing.
func (s *Store) SimulateAdd(idx int, in, out uint64) {
	s.counterIn(idx).Add(in)
	s.counterOut(idx).Add(out)
}

// SetIfaceKernel sets absolute RX/TX bytes from /sys/class/net (matches nload/bpytop).
func (s *Store) SetIfaceKernel(idx int, rxBytes, txBytes uint64) {
	s.counterIn(idx).Store(rxBytes)
	s.counterOut(idx).Store(txBytes)
}
