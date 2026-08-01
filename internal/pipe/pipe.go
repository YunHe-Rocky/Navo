// Package pipe provides Windows Named Pipe transport for IPC.
// Phase 2: JSON-framed messages over named pipes.
// Pipes: \\.\pipe\Navo.UI.Agent.v1, \\.\pipe\Navo.Agent.Service.v1
//
// On non-Windows platforms, this falls back to Unix domain sockets for testing.
package pipe

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// FrameHeader precedes each JSON message frame.
// 4 bytes magic + 4 bytes length (little-endian).
const frameMagic = 0x4E564F50 // "NVOP" = Navo Pipe

// FrameHeaderSize is the size of the frame header in bytes.
const FrameHeaderSize = 8

const maxFramePayload = 10 * 1024 * 1024

// Frame is a length-delimited message frame.
// On the wire: [4 bytes magic][4 bytes length][length bytes JSON payload]

// ReadFrame reads a single frame from a reader.
func ReadFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, FrameHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	magic := binary.LittleEndian.Uint32(header[:4])
	if magic != frameMagic {
		return nil, fmt.Errorf("invalid frame magic: %08x", magic)
	}

	length := binary.LittleEndian.Uint32(header[4:8])
	if length == 0 || length > maxFramePayload {
		return nil, fmt.Errorf("invalid frame length: %d", length)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	return payload, nil
}

// WriteFrame writes a single frame to a writer.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) == 0 || len(payload) > maxFramePayload {
		return fmt.Errorf("invalid payload size: %d bytes", len(payload))
	}

	header := make([]byte, FrameHeaderSize)
	binary.LittleEndian.PutUint32(header[:4], frameMagic)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(payload)))

	if err := writeFull(w, header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if err := writeFull(w, payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// ── Connection ──

// Conn wraps a read/write connection with frame-level I/O.
type Conn struct {
	rwc    io.ReadWriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
	closed bool
}

// NewConn creates a framed connection.
func NewConn(rwc io.ReadWriteCloser) *Conn {
	return &Conn{
		rwc:    rwc,
		reader: bufio.NewReaderSize(rwc, 64*1024),
	}
}

// ReadJSON reads a frame and unmarshals it into v.
func (c *Conn) ReadJSON(v interface{}) error {
	data, err := ReadFrame(c.reader)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// WriteJSON marshals v and writes it as a frame.
func (c *Conn) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return WriteFrame(c.rwc, data)
}

// Close closes the connection.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.rwc.Close()
}

// SetDeadline sets read/write deadlines on the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error {
	if d, ok := c.rwc.(interface{ SetDeadline(time.Time) error }); ok {
		return d.SetDeadline(t)
	}
	return nil
}

// ── Channel (abstract communication endpoint) ──

// Channel represents a bidirectional IPC communication channel.
type Channel struct {
	conn *Conn
	name string
	mu   sync.RWMutex
}

// NewChannel wraps a Conn as a named Channel.
func NewChannel(conn *Conn, name string) *Channel {
	return &Channel{conn: conn, name: name}
}

// Name returns the channel's pipe name.
func (ch *Channel) Name() string { return ch.name }

// Send marshals and sends a message.
func (ch *Channel) Send(msg interface{}) error {
	return ch.conn.WriteJSON(msg)
}

// Receive reads and unmarshals a message.
func (ch *Channel) Receive(v interface{}) error {
	return ch.conn.ReadJSON(v)
}

// Close closes the channel.
func (ch *Channel) Close() error {
	return ch.conn.Close()
}

// SetDeadline sets the deadline for the next read/write.
func (ch *Channel) SetDeadline(t time.Time) error {
	return ch.conn.SetDeadline(t)
}

// ── Listener interface ──

// Listener accepts incoming pipe connections.
type Listener interface {
	// Accept waits for and returns the next connection.
	Accept() (*Channel, error)

	// Close stops listening.
	Close() error

	// Addr returns the listener's pipe name.
	Addr() string

	// PreCreateInstances creates n additional pipe instances ready for clients.
	PreCreateInstances(n int)
}
