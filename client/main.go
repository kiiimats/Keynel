package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"keynel/common"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Keynel Client
//
//  起動: ./client <server-ip>
//       ./client 1.2.3.4
//       ./client 1.2.3.4:7000   (ポートを変えたい場合)
//
//  認証キーは keynel.key から読む（サーバーの keynel.json の client_key をコピー）。
//  または -key フラグで直接指定も可能。
// ─────────────────────────────────────────────────────────────────────────────

var debug bool

func dbg(format string, v ...any) {
	if debug {
		log.Printf(format, v...)
	}
}

// TunnelInfo はサーバーから受け取るトンネル設定。
type TunnelInfo struct {
	ID         string `json:"id"`
	Proto      string `json:"proto"`
	ClientPort int    `json:"client_port"`
	ServerPort int    `json:"server_port"`
	Enabled    bool   `json:"enabled"`
}

type client struct {
	controlAddr string
	dataAddr    string
	key         string
	mu          sync.RWMutex
	tunnels     map[int]*TunnelInfo // serverPort → TunnelInfo
}

func main() {
	flagKey := flag.String("key", "", "認証キー（省略時: keynel.key から読み込み）")
	flagDebug := flag.Bool("debug", false, "デバッグログを表示")
	flag.Parse()

	debug = *flagDebug

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("使い方: client <server-ip>  または  client <server-ip:port>")
	}

	serverAddr := args[0]
	if !strings.Contains(serverAddr, ":") {
		serverAddr = fmt.Sprintf("%s:%d", serverAddr, common.DefaultControlPort)
	}
	host, _, err := net.SplitHostPort(serverAddr)
	if err != nil {
		log.Fatalf("サーバーアドレスのパース失敗: %v", err)
	}
	dataAddr := fmt.Sprintf("%s:%d", host, common.DefaultDataPort)

	// キーの読み込み
	key := *flagKey
	if key == "" {
		data, err := os.ReadFile("keynel.key")
		if err != nil {
			log.Fatal("[Keynel] keynel.key が見つかりません。\nサーバーの keynel.json の client_key の値を keynel.key に書いてください。\nまたは -key フラグで指定できます。")
		}
		key = strings.TrimSpace(string(data))
	}

	log.Printf(" ")
	log.Printf("  Keynel Client | サーバー: %s", serverAddr)
	log.Printf(" ")

	c := &client{
		controlAddr: serverAddr,
		dataAddr:    dataAddr,
		key:         key,
		tunnels:     make(map[int]*TunnelInfo),
	}
	c.loopConnect()
}

func (c *client) loopConnect() {
	for {
		if err := c.connect(); err != nil {
			log.Printf("[control] 切断: %v", err)
		}
		log.Printf("[control] 5秒後に再接続...")
		time.Sleep(5 * time.Second)
	}
}

func (c *client) connect() error {
	conn, err := net.DialTimeout("tcp", c.controlAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("サーバー接続失敗: %w", err)
	}
	defer conn.Close()

	// 認証
	if err := common.WriteLine(conn, "HELLO "+c.key); err != nil {
		return fmt.Errorf("HELLO送信失敗: %w", err)
	}
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	resp, err := common.ReadLine(conn)
	conn.SetDeadline(time.Time{})
	if err != nil {
		return fmt.Errorf("応答受信失敗: %w", err)
	}
	if resp != "OK" {
		return fmt.Errorf("サーバー拒否: %s", resp)
	}

	log.Printf("[control] サーバーに接続しました")

	// PING送信ゴルーチン
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := common.WriteLine(conn, "PING"); err != nil {
				return
			}
		}
	}()

	// メッセージ受信ループ
	for {
		conn.SetDeadline(time.Now().Add(40 * time.Second))
		line, err := common.ReadLine(conn)
		conn.SetDeadline(time.Time{})
		if err != nil {
			return fmt.Errorf("制御接続エラー: %w", err)
		}

		// コマンドと残りを分割
		idx := strings.IndexByte(line, ' ')
		var cmd, rest string
		if idx >= 0 {
			cmd, rest = line[:idx], line[idx+1:]
		} else {
			cmd = line
		}

		switch cmd {
		case "SYNC":
			// SYNC [{"id":...}, ...]
			var list []*TunnelInfo
			if err := json.Unmarshal([]byte(rest), &list); err != nil {
				dbg("[control] SYNC パース失敗: %v", err)
				continue
			}
			c.mu.Lock()
			c.tunnels = make(map[int]*TunnelInfo)
			for _, t := range list {
				if t.Enabled {
					c.tunnels[t.ServerPort] = t
					log.Printf("[tunnel] 登録: %s  サーバー:%d → localhost:%d", t.Proto, t.ServerPort, t.ClientPort)
				}
			}
			c.mu.Unlock()

		case "TUNNEL_ADD", "TUNNEL_UPDATE":
			var t TunnelInfo
			if err := json.Unmarshal([]byte(rest), &t); err != nil {
				dbg("[control] %s パース失敗: %v", cmd, err)
				continue
			}
			c.mu.Lock()
			if t.Enabled {
				c.tunnels[t.ServerPort] = &t
				log.Printf("[tunnel] 追加/更新: %s  サーバー:%d → localhost:%d", t.Proto, t.ServerPort, t.ClientPort)
			} else {
				delete(c.tunnels, t.ServerPort)
				log.Printf("[tunnel] 無効化: サーバー:%d", t.ServerPort)
			}
			c.mu.Unlock()

		case "TUNNEL_DEL":
			// rest = tunnel ID
			c.mu.Lock()
			for port, t := range c.tunnels {
				if t.ID == rest {
					delete(c.tunnels, port)
					log.Printf("[tunnel] 削除: サーバー:%d", port)
					break
				}
			}
			c.mu.Unlock()

		case "CONNECT":
			// CONNECT <conn_id> <server_port>
			parts := strings.Fields(rest)
			if len(parts) == 2 {
				go c.handleTCPConnect(parts[0], parts[1])
			}

		case "CONNECT_UDP":
			parts := strings.Fields(rest)
			if len(parts) == 2 {
				go c.handleUDPConnect(parts[0], parts[1])
			}

		case "PONG":
			// alive

		default:
			dbg("[control] 不明メッセージ: %q", line)
		}
	}
}

// localPortFor はサーバーポートに対応するローカルポート文字列を返す。
func (c *client) localPortFor(serverPortStr string) (string, bool) {
	var serverPort int
	fmt.Sscan(serverPortStr, &serverPort)
	c.mu.RLock()
	t, ok := c.tunnels[serverPort]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d", t.ClientPort), true
}

// ─── TCP ──────────────────────────────────────────────────────────────────────

func (c *client) handleTCPConnect(id, serverPortStr string) {
	localPort, ok := c.localPortFor(serverPortStr)
	if !ok {
		dbg("[tcp] 未知のサーバーポート: %s", serverPortStr)
		return
	}

	sConn, err := net.DialTimeout("tcp", c.dataAddr, 10*time.Second)
	if err != nil {
		dbg("[tcp] データポート接続失敗: %v", err)
		return
	}
	if err := common.WriteLine(sConn, id); err != nil {
		sConn.Close()
		return
	}

	lConn, err := net.DialTimeout("tcp", "localhost:"+localPort, 5*time.Second)
	if err != nil {
		log.Printf("[tcp] ローカル接続失敗 (localhost:%s): %v", localPort, err)
		sConn.Close()
		return
	}

	log.Printf("[tcp] 接続確立  サーバー:%s → localhost:%s", serverPortStr, localPort)
	common.Bridge(sConn, lConn)
	dbg("[tcp] ブリッジ終了 :%s", serverPortStr)
}

// ─── UDP ──────────────────────────────────────────────────────────────────────

func (c *client) handleUDPConnect(id, serverPortStr string) {
	localPort, ok := c.localPortFor(serverPortStr)
	if !ok {
		dbg("[udp] 未知のサーバーポート: %s", serverPortStr)
		return
	}

	sConn, err := net.DialTimeout("tcp", c.dataAddr, 10*time.Second)
	if err != nil {
		dbg("[udp] データポート接続失敗: %v", err)
		return
	}
	defer sConn.Close()

	if err := common.WriteLine(sConn, id); err != nil {
		return
	}

	lConn, err := net.Dial("udp", "localhost:"+localPort)
	if err != nil {
		log.Printf("[udp] ローカル接続失敗 (localhost:%s): %v", localPort, err)
		return
	}
	defer lConn.Close()

	log.Printf("[udp] 接続確立  サーバー:%s → localhost:%s", serverPortStr, localPort)

	// サーバー(TCP) → ローカルUDP
	go func() {
		defer lConn.Close()
		defer sConn.Close()
		for {
			data, err := common.ReadUDPFrame(sConn)
			if err != nil {
				return
			}
			lConn.Write(data)
		}
	}()

	// ローカルUDP → サーバー(TCP)
	buf := make([]byte, common.MaxUDPPacketSize)
	for {
		lConn.SetDeadline(time.Now().Add(120 * time.Second))
		n, err := lConn.Read(buf)
		if err != nil {
			break
		}
		if err := common.WriteUDPFrame(sConn, buf[:n]); err != nil {
			break
		}
	}
	dbg("[udp] セッション終了 :%s", serverPortStr)
}
