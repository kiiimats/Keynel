package main

import (
	"crypto/rand"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"keynel/common"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed all:dashboard_dist
var dashboardFS embed.FS

// ─────────────────────────────────────────────────────────────────────────────
//  Keynel Server
//
//  起動: ./server
//       ./server -debug
//
//  初回起動時に keynel.json を生成してキーを保存する。
//  管理画面API: :7002
//  クライアント制御: :7000
//  データチャンネル: :7001
// ─────────────────────────────────────────────────────────────────────────────

var debug bool

func dbg(format string, v ...any) {
	if debug {
		log.Printf(format, v...)
	}
}

// ─── 設定ファイル ─────────────────────────────────────────────────────────────

type KeynelConfig struct {
	ClientKey string `json:"client_key"`
	APIKey    string `json:"api_key"`
}

const configFile = "keynel.json"

func loadOrCreateConfig() KeynelConfig {
	data, err := os.ReadFile(configFile)
	if err == nil {
		var cfg KeynelConfig
		if json.Unmarshal(data, &cfg) == nil && cfg.ClientKey != "" && cfg.APIKey != "" {
			return cfg
		}
	}
	cfg := KeynelConfig{
		ClientKey: randomHex(16),
		APIKey:    randomHex(16),
	}
	data, _ = json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configFile, data, 0600)
	log.Printf("[Keynel] 設定ファイルを生成しました: %s", configFile)
	return cfg
}

// ─── Tunnel モデル ────────────────────────────────────────────────────────────

type Tunnel struct {
	ID         string    `json:"id"`
	Proto      string    `json:"proto"`       // "tcp" or "udp"
	ClientPort int       `json:"client_port"` // クライアント側ローカルポート
	ServerPort int       `json:"server_port"` // サーバー上の公開ポート
	Enabled    bool      `json:"enabled"`
	RateLimit  bool      `json:"rate_limit"` // DDoS レート制限を有効にするか
	CreatedAt  time.Time `json:"created_at"`
}

// ─── SSEブローカー ────────────────────────────────────────────────────────────

type SSEBroker struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func newSSEBroker() *SSEBroker {
	return &SSEBroker{clients: make(map[chan string]struct{})}
}

func (b *SSEBroker) subscribe() chan string {
	ch := make(chan string, 32)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *SSEBroker) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *SSEBroker) publish(eventType string, data any) {
	payload, _ := json.Marshal(data)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, payload)
	b.mu.Lock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
	b.mu.Unlock()
}

// ─── Server ───────────────────────────────────────────────────────────────────

type Server struct {
	cfg         KeynelConfig
	controlPort int
	dataPort    int
	mgmtPort    int
	guard       *common.IPGuard

	// クライアント接続
	mu      sync.Mutex
	client  net.Conn
	clientW sync.Mutex

	// トンネル管理
	tunnelMu sync.RWMutex
	tunnels  map[string]*Tunnel

	// 公開ポートリスナー管理
	listenerMu sync.Mutex
	listeners  map[string]func() // key: "tcp:10000" → closer func

	// データ接続の待ち合わせ
	pendingMu    sync.Mutex
	pendingConns map[string]chan net.Conn

	// SSE
	broker *SSEBroker
}

func newServer(cfg KeynelConfig, controlPort, dataPort, mgmtPort int) *Server {
	return &Server{
		cfg:          cfg,
		controlPort:  controlPort,
		dataPort:     dataPort,
		mgmtPort:     mgmtPort,
		guard:        common.NewIPGuard(common.DefaultIPGuardConfig),
		tunnels:      make(map[string]*Tunnel),
		listeners:    make(map[string]func()),
		pendingConns: make(map[string]chan net.Conn),
		broker:       newSSEBroker(),
	}
}

func main() {
	flagDebug := flag.Bool("debug", false, "デバッグログを表示")
	flagControl := flag.Int("control", common.DefaultControlPort, "制御ポート")
	flagData := flag.Int("data", common.DefaultDataPort, "データポート")
	flagMgmt := flag.Int("mgmt", common.DefaultMgmtPort, "管理APIポート")
	flag.Parse()

	debug = *flagDebug
	cfg := loadOrCreateConfig()

	log.Printf(" ")
	log.Printf("  Keynel Server")
	log.Printf("  クライアントキー : %s", cfg.ClientKey)
	log.Printf("  ダッシュボードAPIキー: %s", cfg.APIKey)
	log.Printf("  制御  :%d  |  データ :%d  |  管理API :%d", *flagControl, *flagData, *flagMgmt)
	log.Printf(" ")

	srv := newServer(cfg, *flagControl, *flagData, *flagMgmt)
	srv.run()
}

func (s *Server) run() {
	// 制御ポート
	controlLn, err := net.Listen("tcp", fmt.Sprintf(":%d", s.controlPort))
	if err != nil {
		log.Fatalf("[Keynel] 制御ポート :%d Listen失敗: %v", s.controlPort, err)
	}
	// データポート
	dataLn, err := net.Listen("tcp", fmt.Sprintf(":%d", s.dataPort))
	if err != nil {
		log.Fatalf("[Keynel] データポート :%d Listen失敗: %v", s.dataPort, err)
	}

	go s.serveData(dataLn)
	go s.serveMgmt()

	for {
		conn, err := controlLn.Accept()
		if err != nil {
			dbg("[control] accept error: %v", err)
			continue
		}
		go s.handleControl(conn)
	}
}

// ─── クライアント制御接続 ──────────────────────────────────────────────────────

func (s *Server) handleControl(conn net.Conn) {
	defer conn.Close()
	addr := conn.RemoteAddr().String()
	dbg("[control] 接続試行: %s", addr)

	conn.SetDeadline(time.Now().Add(15 * time.Second))
	line, err := common.ReadLine(conn)
	conn.SetDeadline(time.Time{})
	if err != nil {
		dbg("[control] 認証受信失敗: %v", err)
		return
	}

	parts := strings.Fields(line)
	if len(parts) != 2 || parts[0] != "HELLO" {
		common.WriteLine(conn, "ERROR bad_request")
		return
	}
	if parts[1] != s.cfg.ClientKey {
		common.WriteLine(conn, "ERROR invalid_key")
		log.Printf("[control] 認証失敗 (%s): キー不一致", addr)
		return
	}

	s.clientW.Lock()
	common.WriteLine(conn, "OK")
	s.clientW.Unlock()

	log.Printf("[control] クライアント接続: %s", addr)
	s.broker.publish("client_status", map[string]any{"connected": true, "addr": addr})

	s.setClient(conn)
	defer func() {
		s.clearClient(conn)
		log.Printf("[control] クライアント切断: %s", addr)
		s.broker.publish("client_status", map[string]any{"connected": false})
	}()

	// 接続時に有効なトンネル一覧を送信
	s.syncTunnelsToClient()

	// PINGループ
	for {
		conn.SetDeadline(time.Now().Add(40 * time.Second))
		line, err := common.ReadLine(conn)
		conn.SetDeadline(time.Time{})
		if err != nil {
			dbg("[control] 受信エラー: %v", err)
			return
		}
		switch line {
		case "PING":
			s.clientW.Lock()
			common.WriteLine(conn, "PONG")
			s.clientW.Unlock()
		default:
			dbg("[control] 不明メッセージ: %q", line)
		}
	}
}

// syncTunnelsToClient は有効なトンネルをすべてクライアントに送る。
func (s *Server) syncTunnelsToClient() {
	s.tunnelMu.RLock()
	list := make([]*Tunnel, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		list = append(list, t)
	}
	s.tunnelMu.RUnlock()

	data, _ := json.Marshal(list)
	s.sendToClient("SYNC " + string(data))

	// リスナーも起動
	for _, t := range list {
		if t.Enabled {
			go s.openListener(t)
		}
	}
}

func (s *Server) setClient(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		s.client.Close()
	}
	s.client = conn
}

func (s *Server) clearClient(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == conn {
		s.client = nil
	}
}

func (s *Server) sendToClient(msg string) error {
	s.mu.Lock()
	c := s.client
	s.mu.Unlock()
	if c == nil {
		return fmt.Errorf("クライアント未接続")
	}
	s.clientW.Lock()
	defer s.clientW.Unlock()
	return common.WriteLine(c, msg)
}

func (s *Server) isClientConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client != nil
}

// ─── トンネル管理 ──────────────────────────────────────────────────────────────

func (s *Server) addTunnel(t *Tunnel) {
	s.tunnelMu.Lock()
	s.tunnels[t.ID] = t
	s.tunnelMu.Unlock()

	data, _ := json.Marshal(t)
	s.sendToClient("TUNNEL_ADD " + string(data))

	if t.Enabled {
		go s.openListener(t)
	}
	s.broker.publish("tunnel_add", t)
}

func (s *Server) updateTunnel(id string, enabled bool, rateLimit bool) *Tunnel {
	s.tunnelMu.Lock()
	t, ok := s.tunnels[id]
	if !ok {
		s.tunnelMu.Unlock()
		return nil
	}
	t.Enabled = enabled
	t.RateLimit = rateLimit
	s.tunnelMu.Unlock()

	data, _ := json.Marshal(t)
	s.sendToClient("TUNNEL_UPDATE " + string(data))

	if enabled {
		go s.openListener(t)
	} else {
		s.closeListener(t.Proto, t.ServerPort)
	}
	s.broker.publish("tunnel_update", t)
	return t
}

func (s *Server) deleteTunnel(id string) bool {
	s.tunnelMu.Lock()
	t, ok := s.tunnels[id]
	if !ok {
		s.tunnelMu.Unlock()
		return false
	}
	delete(s.tunnels, id)
	s.tunnelMu.Unlock()

	s.sendToClient("TUNNEL_DEL " + id)
	s.closeListener(t.Proto, t.ServerPort)
	s.broker.publish("tunnel_delete", map[string]string{"id": id})
	return true
}

// ─── 公開ポートリスナー ────────────────────────────────────────────────────────

func listenerKey(proto string, port int) string {
	return fmt.Sprintf("%s:%d", proto, port)
}

func (s *Server) openListener(t *Tunnel) {
	key := listenerKey(t.Proto, t.ServerPort)

	s.listenerMu.Lock()
	if _, exists := s.listeners[key]; exists {
		s.listenerMu.Unlock()
		dbg("[listener] %s すでに公開中", key)
		return
	}
	// プレースホルダーを入れて二重起動を防ぐ
	s.listeners[key] = func() {}
	s.listenerMu.Unlock()

	if t.Proto == "tcp" {
		s.openTCPListener(t, key)
	} else {
		s.openUDPListener(t, key)
	}
}

func (s *Server) closeListener(proto string, port int) {
	key := listenerKey(proto, port)
	s.listenerMu.Lock()
	closer, ok := s.listeners[key]
	if ok {
		delete(s.listeners, key)
	}
	s.listenerMu.Unlock()
	if ok && closer != nil {
		closer()
	}
}

func (s *Server) openTCPListener(t *Tunnel, key string) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", t.ServerPort))
	if err != nil {
		log.Printf("[tcp] :%d Listen失敗: %v", t.ServerPort, err)
		s.listenerMu.Lock()
		delete(s.listeners, key)
		s.listenerMu.Unlock()
		return
	}

	closer := func() { ln.Close() }
	s.listenerMu.Lock()
	s.listeners[key] = closer
	s.listenerMu.Unlock()

	log.Printf("[tcp] :%d 公開開始 → client:%d", t.ServerPort, t.ClientPort)

	go func() {
		defer func() {
			ln.Close()
			s.listenerMu.Lock()
			delete(s.listeners, key)
			s.listenerMu.Unlock()
			log.Printf("[tcp] :%d 公開終了", t.ServerPort)
		}()
		for {
			conn, err := ln.Accept()
			if err != nil {
				dbg("[tcp] :%d accept error: %v", t.ServerPort, err)
				return
			}
			go s.handleTCPPublic(conn, t)
		}
	}()
}

func (s *Server) handleTCPPublic(extConn net.Conn, t *Tunnel) {
	if t.RateLimit {
		if ok, reason := s.guard.Allow(extConn.RemoteAddr()); !ok {
			log.Printf("[guard] TCP :%d 遮断 %s (%s)", t.ServerPort, extConn.RemoteAddr(), reason)
			extConn.Close()
			return
		}
		defer s.guard.Release(extConn.RemoteAddr())
	}

	// トンネルがまだ有効か確認
	s.tunnelMu.RLock()
	enabled := t.Enabled
	s.tunnelMu.RUnlock()
	if !enabled {
		extConn.Close()
		return
	}

	if !s.isClientConnected() {
		log.Printf("[tcp] クライアント未接続 → 拒否 :%d", t.ServerPort)
		extConn.Close()
		return
	}

	id := newID()
	ch := make(chan net.Conn, 1)
	s.pendingMu.Lock()
	s.pendingConns[id] = ch
	s.pendingMu.Unlock()

	if err := s.sendToClient(fmt.Sprintf("CONNECT %s %d", id, t.ServerPort)); err != nil {
		dbg("[tcp] CONNECT送信失敗: %v", err)
		extConn.Close()
		s.pendingMu.Lock()
		delete(s.pendingConns, id)
		s.pendingMu.Unlock()
		return
	}

	select {
	case dataConn := <-ch:
		log.Printf("[tcp] 接続確立 :%d  %s", t.ServerPort, extConn.RemoteAddr())
		common.Bridge(extConn, dataConn)
		dbg("[tcp] ブリッジ終了 :%d", t.ServerPort)
	case <-time.After(10 * time.Second):
		dbg("[tcp] タイムアウト :%d", t.ServerPort)
		extConn.Close()
		s.pendingMu.Lock()
		delete(s.pendingConns, id)
		s.pendingMu.Unlock()
	}
}

func (s *Server) openUDPListener(t *Tunnel, key string) {
	pc, err := net.ListenPacket("udp", fmt.Sprintf(":%d", t.ServerPort))
	if err != nil {
		log.Printf("[udp] :%d Listen失敗: %v", t.ServerPort, err)
		s.listenerMu.Lock()
		delete(s.listeners, key)
		s.listenerMu.Unlock()
		return
	}

	closer := func() { pc.Close() }
	s.listenerMu.Lock()
	s.listeners[key] = closer
	s.listenerMu.Unlock()

	log.Printf("[udp] :%d 公開開始 → client:%d", t.ServerPort, t.ClientPort)

	go func() {
		defer func() {
			pc.Close()
			s.listenerMu.Lock()
			delete(s.listeners, key)
			s.listenerMu.Unlock()
			log.Printf("[udp] :%d 公開終了", t.ServerPort)
		}()

		var sessMu sync.Mutex
		sessions := make(map[string]*udpSession)
		buf := make([]byte, common.MaxUDPPacketSize)

		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				dbg("[udp] :%d ReadFrom error: %v", t.ServerPort, err)
				return
			}
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			addrKey := addr.String()

			sessMu.Lock()
			sess, exists := sessions[addrKey]
			if !exists {
				// DDoS チェック（トンネルのレート制限が有効な場合のみ）
				if t.RateLimit {
					if ok, reason := s.guard.Allow(addr); !ok {
						sessMu.Unlock()
						log.Printf("[guard] UDP :%d 遮断 %s (%s)", t.ServerPort, addrKey, reason)
						continue
					}
				}
				sess = &udpSession{remoteAddr: addr}
				sessions[addrKey] = sess
				sessMu.Unlock()

				sess.deliver(pkt)

				id := newID()
				ch := make(chan net.Conn, 1)
				s.pendingMu.Lock()
				s.pendingConns[id] = ch
				s.pendingMu.Unlock()

				if err := s.sendToClient(fmt.Sprintf("CONNECT_UDP %s %d", id, t.ServerPort)); err != nil {
					dbg("[udp] CONNECT_UDP送信失敗: %v", err)
					s.pendingMu.Lock()
					delete(s.pendingConns, id)
					s.pendingMu.Unlock()
					sessMu.Lock()
					delete(sessions, addrKey)
					sessMu.Unlock()
					if t.RateLimit {
						s.guard.Release(addr)
					}
					continue
				}

				go func(sess *udpSession, addrKey, id string) {
					if t.RateLimit {
						defer s.guard.Release(addr)
					}
					var dataConn net.Conn
					select {
					case dataConn = <-ch:
					case <-time.After(10 * time.Second):
						s.pendingMu.Lock()
						delete(s.pendingConns, id)
						s.pendingMu.Unlock()
						sessMu.Lock()
						delete(sessions, addrKey)
						sessMu.Unlock()
						return
					}
					log.Printf("[udp] 接続確立 :%d  %s", t.ServerPort, addrKey)
					sess.setDataConn(dataConn)
					defer func() {
						sess.close()
						sessMu.Lock()
						delete(sessions, addrKey)
						sessMu.Unlock()
					}()
					for {
						data, err := common.ReadUDPFrame(dataConn)
						if err != nil {
							return
						}
						pc.WriteTo(data, sess.remoteAddr)
					}
				}(sess, addrKey, id)
			} else {
				sessMu.Unlock()
				sess.deliver(pkt)
			}
		}
	}()
}

// ─── UDP セッション ────────────────────────────────────────────────────────────

type udpSession struct {
	remoteAddr net.Addr
	mu         sync.Mutex
	dataConn   net.Conn
	queue      [][]byte
	closed     bool
}

func (sess *udpSession) deliver(pkt []byte) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.closed {
		return
	}
	if sess.dataConn == nil {
		if len(sess.queue) < 256 {
			cp := make([]byte, len(pkt))
			copy(cp, pkt)
			sess.queue = append(sess.queue, cp)
		}
		return
	}
	common.WriteUDPFrame(sess.dataConn, pkt)
}

func (sess *udpSession) setDataConn(dc net.Conn) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.dataConn = dc
	for _, p := range sess.queue {
		common.WriteUDPFrame(dc, p)
	}
	sess.queue = nil
}

func (sess *udpSession) close() {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.closed = true
	if sess.dataConn != nil {
		sess.dataConn.Close()
	}
}

// ─── データポート ──────────────────────────────────────────────────────────────

func (s *Server) serveData(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			dbg("[data] accept error: %v", err)
			continue
		}
		go s.handleData(conn)
	}
}

func (s *Server) handleData(conn net.Conn) {
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	id, err := common.ReadLine(conn)
	conn.SetDeadline(time.Time{})
	if err != nil {
		conn.Close()
		return
	}
	id = strings.TrimSpace(id)

	s.pendingMu.Lock()
	ch, ok := s.pendingConns[id]
	if ok {
		delete(s.pendingConns, id)
	}
	s.pendingMu.Unlock()

	if !ok {
		dbg("[data] 不明なid: %.8s", id)
		conn.Close()
		return
	}
	ch <- conn
}

// ─── 管理HTTP API ──────────────────────────────────────────────────────────────

func (s *Server) serveMgmt() {
	mux := http.NewServeMux()

	// API（認証必要）
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/status", s.handleStatus)
	apiMux.HandleFunc("/api/tunnels", s.handleTunnels)
	apiMux.HandleFunc("/api/tunnels/", s.handleTunnelByID)
	apiMux.HandleFunc("/api/events", s.handleSSE)
	mux.Handle("/api/", s.authMiddleware(apiMux))

	// ダッシュボード静的ファイル配信（embed）
	sub, err := fs.Sub(dashboardFS, "dashboard_dist")
	if err != nil {
		log.Fatalf("[mgmt] embed失敗: %v", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// ファイルが存在しない場合は SPA フォールバック
		f, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			r2 := *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, &r2)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
	log.Printf("[mgmt] ダッシュボード配信: embed")

	addr := fmt.Sprintf(":%d", s.mgmtPort)
	log.Printf("[mgmt] 管理API起動: %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[mgmt] HTTP起動失敗: %v", err)
	}
}

// corsMiddleware はCloudflare Pages等からのクロスオリジンリクエストを許可する。
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware はX-API-Keyヘッダーを検証する（SSEは除外）。
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSEはEventsource APIがカスタムヘッダーを送れないのでクエリパラメータも許可
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("key")
		}
		if apiKey != s.cfg.APIKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// GET /api/status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.tunnelMu.RLock()
	count := len(s.tunnels)
	s.tunnelMu.RUnlock()
	total, _, banned := s.guard.Stats()
	writeJSON(w, 200, map[string]any{
		"client_connected": s.isClientConnected(),
		"tunnel_count":     count,
		"active_conns":     total,
		"banned_ips":       banned,
	})
}

// GET /api/tunnels  → トンネル一覧
// POST /api/tunnels → トンネル作成
func (s *Server) handleTunnels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.tunnelMu.RLock()
		list := make([]*Tunnel, 0, len(s.tunnels))
		for _, t := range s.tunnels {
			list = append(list, t)
		}
		s.tunnelMu.RUnlock()
		writeJSON(w, 200, list)

	case http.MethodPost:
		var req struct {
			Proto      string `json:"proto"`
			ClientPort int    `json:"client_port"`
			ServerPort int    `json:"server_port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, 400)
			return
		}
		if req.Proto != "tcp" && req.Proto != "udp" {
			http.Error(w, `{"error":"proto must be tcp or udp"}`, 400)
			return
		}
		if req.ClientPort <= 0 || req.ClientPort > 65535 ||
			req.ServerPort <= 0 || req.ServerPort > 65535 {
			http.Error(w, `{"error":"invalid port"}`, 400)
			return
		}
		t := &Tunnel{
			ID:         newID()[:8],
			Proto:      req.Proto,
			ClientPort: req.ClientPort,
			ServerPort: req.ServerPort,
			Enabled:    true,
			CreatedAt:  time.Now(),
		}
		s.addTunnel(t)
		log.Printf("[mgmt] トンネル追加: %s %d → %d", t.Proto, t.ClientPort, t.ServerPort)
		writeJSON(w, 201, t)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

// PATCH /api/tunnels/<id>  → ON/OFF
// DELETE /api/tunnels/<id> → 削除
func (s *Server) handleTunnelByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tunnels/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var req struct {
			Enabled   bool `json:"enabled"`
			RateLimit bool `json:"rate_limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, 400)
			return
		}
		t := s.updateTunnel(id, req.Enabled, req.RateLimit)
		if t == nil {
			http.Error(w, `{"error":"not found"}`, 404)
			return
		}
		log.Printf("[mgmt] トンネル更新: %s enabled=%v ratelimit=%v", id, req.Enabled, req.RateLimit)
		writeJSON(w, 200, t)

	case http.MethodDelete:
		if !s.deleteTunnel(id) {
			http.Error(w, `{"error":"not found"}`, 404)
			return
		}
		log.Printf("[mgmt] トンネル削除: %s", id)
		writeJSON(w, 200, map[string]string{"id": id})

	default:
		http.Error(w, "method not allowed", 405)
	}
}

// GET /api/events → SSE
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)

	ch := s.broker.subscribe()
	defer s.broker.unsubscribe(ch)

	// 初期状態を送信
	s.tunnelMu.RLock()
	list := make([]*Tunnel, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		list = append(list, t)
	}
	s.tunnelMu.RUnlock()

	initData, _ := json.Marshal(map[string]any{
		"tunnels":          list,
		"client_connected": s.isClientConnected(),
	})
	fmt.Fprintf(w, "event: init\ndata: %s\n\n", initData)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-ticker.C:
			// keep-alive
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// ─── ユーティリティ ────────────────────────────────────────────────────────────

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
