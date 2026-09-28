// mpvsync - minimal peer-to-peer watch-together sync for mpv and libmpv based players.
//
//	mpvsync -ipc <mpv-ipc> -peers <ip[:port],...> [-port 7373]   (started by mpvsync.lua)
//
// Stateless UDP: when the local user pauses / plays / seeks, the new state is sent to every peer, which
// applies it if it plays the same file (same file name and size). Once a second a heartbeat with the position
// is sent too; whoever is more than 2s behind a playing peer seeks forward to it, and whoever has just
// opened the file while a peer is paused pauses at its position. Switching to another file takes the peers
// along if they have that file in their playlist; a peer without it replies, and the switcher pauses to wait.
// Nothing is remembered.
package main

import (
	"flag"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Pkt is one UDP datagram (JSON): a user action (Op) or a heartbeat.
type Pkt struct {
	Op     bool    `json:"op,omitempty"`
	Seek   bool    `json:"seek,omitempty"`
	Load   bool    `json:"load,omitempty"` // the user switched to file Name
	Miss   bool    `json:"miss,omitempty"` // reply to Load: file Name is not in my playlist
	File   string  `json:"file"`           // hash of file name + size
	Name   string  `json:"name,omitempty"` // url-encoded file name, only with Load / Miss
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

// fileName: the file name of a local path or URL (query string such as ?sign=... dropped), Unicode-normalized
// and url-encoded into one canonical form, so every peer derives the same name for the same file.
func fileName(p string) string {
	if p == "" {
		return ""
	}
	name := filepath.Base(p)
	if u, err := url.Parse(p); err == nil && len(u.Scheme) > 1 { // a URL, not a drive letter
		name = path.Base(u.Path)
	}
	return url.PathEscape(norm.NFC.String(name))
}

// show: a file name for the OSD.
func show(name string) string {
	if s, err := url.PathUnescape(name); err == nil {
		return s
	}
	return name
}

func fmtTime(t float64) string {
	s := int(math.Max(t, 0) + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}
