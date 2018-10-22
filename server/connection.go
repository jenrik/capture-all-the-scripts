package server

import (
	"net"
	"sync/atomic"
	"time"
)

// Connection wraps net.Conn but counts the total number of written bytes
type Connection struct {
	net.Conn

	Started   time.Time
	SessionID string
	Remote    string

	written uint64
}

func (c *Connection) Write(b []byte) (int, error) {
	err := c.Conn.SetWriteDeadline(time.Now().Add(time.Minute * 5))
	if err != nil {
		return 0, err
	}
	n, err := c.Conn.Write(b)
	atomic.AddUint64(&c.written, uint64(n))
	return n, err
}

func (c *Connection) Written() uint64 {
	return atomic.LoadUint64(&c.written)
}
