// mpvsync - minimal peer-to-peer watch-together sync for mpv and libmpv based players.
//
//	mpvsync -ipc <mpv-ipc> -peers <ip[:port],...> [-port 7373]   (started by mpvsync.lua)
//
// Stateless UDP: when the local user pauses / plays / seeks, the new state is sent to every peer, which
// applies it if it plays the same file (same size in bytes; duration if the size is unknown). Once a second a heartbeat with the position
// is sent too; whoever is more than 2s behind a playing peer seeks forward to it, and whoever has just
// opened the file while a peer is paused pauses at its position. Nothing is remembered.
package main

import (
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
)

// Pkt is one UDP datagram (JSON): a user action (Op) or a heartbeat.
type Pkt struct {
	Op     bool    `json:"op,omitempty"`
	Seek   bool    `json:"seek,omitempty"`
	Size   int64   `json:"size,omitempty"` // file size in bytes: identifies the file
	Dur    float64 `json:"dur"`
	Pos    float64 `json:"pos"`
	Paused bool    `json:"paused"`
}

func main() {
	ipc := flag.String("ipc", "", "mpv --input-ipc-server path")
	peers := flag.String("peers", "", "peer addresses, ip or ip:port, comma separated")
	port := flag.Int("port", 7373, "UDP port")
	flag.Parse()
	if *ipc == "" || *peers == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*ipc, parsePeers(*peers, *port), *port); err != nil {
		fmt.Fprintln(os.Stderr, "mpvsync:", err)
		os.Exit(1)
	}
}

func parsePeers(list string, port int) []net.Addr {
	var out []net.Addr
	for _, s := range strings.Split(list, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(s); err != nil {
			s = net.JoinHostPort(s, strconv.Itoa(port))
		}
		if a, err := net.ResolveUDPAddr("udp4", s); err == nil {
			out = append(out, a)
		}
	}
	return out
}

func fmtTime(t float64) string {
	s := int(math.Max(t, 0) + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}
