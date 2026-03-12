// Package common はサーバー・クライアント共通のプロトコル定義を提供します。
//
// ─── 制御チャンネル（テキスト、改行区切り）────────────────────────────────────
//
// [Client → Server]
//   HELLO <key>\n
//   PING\n
//
// [Server → Client]
//   OK\n
//   ERROR <reason>\n
//   SYNC <json_tunnels_array>\n          — 接続時に全トンネル設定を送る
//   TUNNEL_ADD <json_tunnel>\n           — トンネル追加
//   TUNNEL_DEL <id>\n                   — トンネル削除
//   TUNNEL_UPDATE <json_tunnel>\n        — トンネル更新（ON/OFF）
//   CONNECT <conn_id> <server_port>\n   — TCP接続が来た
//   CONNECT_UDP <conn_id> <server_port>\n
//   PONG\n
//
// ─── データチャンネル ─────────────────────────────────────────────────────────
//   クライアント → サーバー: "<conn_id>\n" → 生バイトブリッジ (TCP)
//   クライアント → サーバー: "<conn_id>\n" → フレーム形式 (UDP)
//   UDPフレーム: [2byte big-endian length][data]

package common

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
)

const (
	DefaultControlPort = 7000
	DefaultDataPort    = 7001
	DefaultMgmtPort    = 7002
	MaxUDPPacketSize   = 65535
)

func WriteLine(conn net.Conn, line string) error {
	_, err := fmt.Fprintln(conn, line)
	return err
}

func ReadLine(conn net.Conn) (string, error) {
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			return "", err
		}
		if buf[0] == '\n' {
			break
		}
		if buf[0] != '\r' {
			sb.WriteByte(buf[0])
		}
		if sb.Len() > 65536 {
			return "", fmt.Errorf("line too long")
		}
	}
	return sb.String(), nil
}

func WriteUDPFrame(conn net.Conn, data []byte) error {
	buf := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(buf, uint16(len(data)))
	copy(buf[2:], data)
	_, err := conn.Write(buf)
	return err
}

func ReadUDPFrame(conn net.Conn) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	data := make([]byte, binary.BigEndian.Uint16(header))
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}
	return data, nil
}
