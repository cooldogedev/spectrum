package api

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/cooldogedev/spectrum/api/packet"
	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/protocol"
)

type Client struct {
	conn net.Conn
	pool packet.Pool

	writer *protocol.Writer
	reader *protocol.Reader
}

func NewClient(conn net.Conn, pool packet.Pool) *Client {
	return &Client{
		conn: conn,
		pool: pool,

		reader: protocol.NewReader(conn),
		writer: protocol.NewWriter(conn),
	}
}

func (c *Client) ReadPacket() (pk packet.Packet, err error) {
	payload, err := c.reader.ReadPacket()
	if err != nil {
		return nil, err
	}

	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Write(payload)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)

		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet: %v", r)
		}
	}()

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

func (c *Client) WritePacket(pk packet.Packet) error {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	if err := binary.Write(buf, binary.LittleEndian, pk.ID()); err != nil {
		return err
	}

	pk.Encode(buf)
	return c.writer.Write(buf.Bytes())
}

func (c *Client) Close() error {
	return c.conn.Close()
}
