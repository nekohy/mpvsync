package mpv

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// dial opens the named pipe with overlapped I/O, so reads and writes do not block each other.
// mpv creates the pipe asynchronously, so retry for a moment.
func dial(path string) (net.Conn, error) {
	timeout := 5 * time.Second
	var err error
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		var c net.Conn
		if c, err = winio.DialPipe(path, &timeout); err == nil {
			return c, nil
		}
	}
	return nil, err
}
