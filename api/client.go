package api

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/protocol"
)

var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 128))
	},
}

// Client represents a connection to the API service, managing packet reading and writing
// over an underlying net.Conn.
type Client struct {
	conn   net.Conn
	pool   packet.Pool
	writer *protocol.Writer
	reader *protocol.Reader
	mu     sync.Mutex
}

// newClient creates a new Client instance using the provided net.Conn.
// It is used for reading and writing packets to the underlying connection.
func newClient(conn net.Conn, pool packet.Pool) *Client {
	return &Client{
		conn: conn,
		pool: pool,

		reader: protocol.NewReader(conn),
		writer: protocol.NewWriter(conn),
	}
}

// ReadPacket reads the next available packet from the connection and decodes it.
func (c *Client) ReadPacket() (pk packet.Packet, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet: %v", r)
		}
	}()

	payload, err := c.reader.ReadPacket()
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(payload)
	var packetID uint32
	if err := binary.Read(buf, binary.LittleEndian, &packetID); err != nil {
		return nil, err
	}

	factory, ok := c.pool[packetID]
	if !ok {
		return nil, fmt.Errorf("unknown packet ID: %v", packetID)
	}
	pk = factory()
	pk.Decode(buf)
	return
}

// WritePacket encodes and writes the provided packet to the underlying connection.
func (c *Client) WritePacket(pk packet.Packet) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if err := binary.Write(buf, binary.LittleEndian, pk.ID()); err != nil {
		return err
	}
	pk.Encode(buf)
	return c.writer.Write(buf.Bytes())
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
