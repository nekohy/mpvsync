//go:build !windows

package mpv

import "net"

func dial(path string) (net.Conn, error) { return net.Dial("unix", path) }
