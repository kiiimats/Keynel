package common

import (
	"net"
	"sync"
	"time"
)

type IPGuardConfig struct {
	RateWindow    time.Duration
	RateMaxConns  int
	MaxConnsPerIP int
	MaxConnsTotal int
	BanThreshold  int
	BanDuration   time.Duration
}

var DefaultIPGuardConfig = IPGuardConfig{
	RateWindow:    10 * time.Second,
	RateMaxConns:  30,
	MaxConnsPerIP: 20,
	MaxConnsTotal: 1000,
	BanThreshold:  3,
	BanDuration:   5 * time.Minute,
}

type ipState struct {
	timestamps  []time.Time
	violations  int
	bannedUntil time.Time
	activeCons  int
}

type IPGuard struct {
	cfg   IPGuardConfig
	mu    sync.Mutex
	ips   map[string]*ipState
	total int
}

func NewIPGuard(cfg IPGuardConfig) *IPGuard {
	g := &IPGuard{cfg: cfg, ips: make(map[string]*ipState)}
	go g.cleanup()
	return g
}

func (g *IPGuard) Allow(addr net.Addr) (bool, string) {
	ip := extractIP(addr)
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.total >= g.cfg.MaxConnsTotal {
		return false, "全体接続数上限"
	}
	st := g.getOrCreate(ip)
	if now.Before(st.bannedUntil) {
		return false, "BAN中(" + st.bannedUntil.Sub(now).Round(time.Second).String() + ")"
	}
	cutoff := now.Add(-g.cfg.RateWindow)
	valid := st.timestamps[:0]
	for _, t := range st.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	st.timestamps = valid
	if len(st.timestamps) >= g.cfg.RateMaxConns {
		st.violations++
		if st.violations >= g.cfg.BanThreshold {
			st.bannedUntil = now.Add(g.cfg.BanDuration)
			return false, "レート超過BAN"
		}
		return false, "レート制限"
	}
	if st.activeCons >= g.cfg.MaxConnsPerIP {
		return false, "IP同時接続数上限"
	}
	st.timestamps = append(st.timestamps, now)
	st.activeCons++
	g.total++
	return true, ""
}

func (g *IPGuard) Release(addr net.Addr) {
	ip := extractIP(addr)
	g.mu.Lock()
	defer g.mu.Unlock()
	if st, ok := g.ips[ip]; ok && st.activeCons > 0 {
		st.activeCons--
	}
	if g.total > 0 {
		g.total--
	}
}

func (g *IPGuard) Stats() (total, uniqueIPs, banned int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	for _, st := range g.ips {
		if now.Before(st.bannedUntil) {
			banned++
		}
	}
	return g.total, len(g.ips), banned
}

func (g *IPGuard) getOrCreate(ip string) *ipState {
	if st, ok := g.ips[ip]; ok {
		return st
	}
	st := &ipState{}
	g.ips[ip] = st
	return st
}

func (g *IPGuard) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		g.mu.Lock()
		for ip, st := range g.ips {
			if st.activeCons == 0 && now.After(st.bannedUntil) &&
				(len(st.timestamps) == 0 || now.Sub(st.timestamps[len(st.timestamps)-1]) > 10*time.Minute) {
				delete(g.ips, ip)
			}
		}
		g.mu.Unlock()
	}
}

func extractIP(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
