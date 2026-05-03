package dewy

import (
	"fmt"
	"io"
	"net"
	"time"
)

// limitedWriter wraps an io.Writer and limits the total bytes written.
// Returns an error when the limit is exceeded.
type limitedWriter struct {
	W       io.Writer
	N       int64
	written int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written+int64(len(p)) > lw.N {
		return 0, fmt.Errorf("write limit exceeded: maximum %d bytes", lw.N)
	}
	n, err := lw.W.Write(p)
	lw.written += int64(n)
	return n, err
}

// timeoutConn wraps net.Conn and resets the deadline on every read/write.
type timeoutConn struct {
	net.Conn
	idleTimeout time.Duration
}

func (c *timeoutConn) Read(b []byte) (int, error) {
	if err := c.SetDeadline(time.Now().Add(c.idleTimeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}

func (c *timeoutConn) Write(b []byte) (int, error) {
	if err := c.SetDeadline(time.Now().Add(c.idleTimeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(b)
}
