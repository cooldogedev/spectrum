package server

import (
	"bytes"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/spectrum-proxy/spectrum/internal"
	proto "github.com/spectrum-proxy/spectrum/protocol"
	packet2 "github.com/spectrum-proxy/spectrum/server/packet"
	"net"
	"sync"
	"sync/atomic"
)

// Conn is a connection to a server. It is used to read and write packets to the server, and to manage the
// connection to the server.
type Conn struct {
	conn       quic.Connection
	compressor packet.Compression

	reader *proto.Reader
	writer *proto.Writer

	readMu  sync.Mutex
	writeMu sync.Mutex

	gameData minecraft.GameData
	shieldID atomic.Int32

	closed chan struct{}

	pool            packet.Pool
	header          packet.Header
	deferredPackets []packet.Packet
}

// NewConn creates a new Conn with the innerConn and pool passed.
func NewConn(conn quic.Connection, stream quic.Stream, pool packet.Pool) *Conn {
	c := &Conn{
		conn:       conn,
		compressor: packet.FlateCompression,

		reader: proto.NewReader(stream),
		writer: proto.NewWriter(stream),

		closed: make(chan struct{}),

		pool:     pool,
		header:   packet.Header{},
		shieldID: atomic.Int32{},
	}

	go func() {
		for {
			select {
			case <-c.closed:
				return
			default:
				if err := c.reader.Read(); err != nil {
					return
				}
			}
		}
	}()
	return c
}

// ReadPacket reads a packet from the connection. It returns the packet read, or an error if the packet could not
// be read.
func (c *Conn) ReadPacket() (pk packet.Packet, err error) {
	select {
	case <-c.closed:
		return nil, fmt.Errorf("connection closed")
	default:
		return c.read()
	}
}

// ReadDeferred reads all packets buffered in the connection and returns them. It returns an empty slice if no
// packets were buffered.
func (c *Conn) ReadDeferred() []packet.Packet {
	packets := c.deferredPackets
	c.deferredPackets = nil
	return packets
}

// WritePacket writes a packet to the connection. It returns an error if the packet could not be written.
func (c *Conn) WritePacket(pk packet.Packet) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	c.header.PacketID = pk.ID()
	if err := c.header.Write(buf); err != nil {
		return err
	}

	pk.Marshal(protocol.NewWriter(buf, c.shieldID.Load()))

	data, err := c.compressor.Compress(buf.Bytes())
	if err != nil {
		return err
	}
	return c.writer.Write(data)
}

// Expect reads a packet from the connection and expects it to have the ID passed. If the packet read does not
// have the ID passed, it will be deferred and the function will be called again until a packet with the ID
// passed is read. It returns the packet read, or an error if the packet could not be read.
func (c *Conn) Expect(id uint32, deferrable bool) (packet.Packet, error) {
	pk, err := c.ReadPacket()
	if err != nil {
		return nil, err
	}

	if pk.ID() != id {
		c.deferredPackets = append(c.deferredPackets, pk)
		return c.Expect(id, deferrable)
	}

	if deferrable {
		c.deferredPackets = append(c.deferredPackets, pk)
	}
	return pk, nil
}

// SetShieldID sets the shield ID of the connection. It is used to set the shield ID of the connection, which is
// used to read and write packets.
func (c *Conn) SetShieldID(id int32) {
	c.shieldID.Store(id)
}

// GameData ...
func (c *Conn) GameData() minecraft.GameData {
	return c.gameData
}

// LocalAddr ...
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// RemoteAddr ...
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Close ...
func (c *Conn) Close() {
	select {
	case <-c.closed:
		return
	default:
		close(c.closed)
		_ = c.conn.CloseWithError(0, "")
	}
}

func (c *Conn) read() (pk packet.Packet, err error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	data, err := c.compressor.Decompress(c.reader.ReadPacket())
	if err != nil {
		return nil, err
	}

	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Write(data)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)

		if r := recover(); r != nil {
			err = fmt.Errorf("panic while reading packet: %v", r)
		}
	}()

	header := packet.Header{}
	if err := header.Read(buf); err != nil {
		return nil, err
	}

	factory, ok := c.pool[header.PacketID]
	if !ok {
		return nil, fmt.Errorf("unknown packet ID %v", header.PacketID)
	}

	pk = factory()
	pk.Marshal(protocol.NewReader(buf, c.shieldID.Load(), false))
	return pk, nil
}

// login logs the connection into the server with the address, clientData and identityData passed. It returns
// an error if the connection could not be logged in.
func (c *Conn) login(addr string, clientData login.ClientData, identityData login.IdentityData) error {
	err := c.WritePacket(&packet2.Connect{
		Addr:     addr,
		EntityID: computeEntityID(identityData.XUID),

		ClientData:   clientData,
		IdentityData: identityData,
	})
	if err != nil {
		return fmt.Errorf("failed to write connect packet: %v", err)
	}

	startGamePacket, err := c.Expect(packet.IDStartGame, false)
	if err != nil {
		return fmt.Errorf("failed to read start game packet: %v", err)
	}

	err = c.WritePacket(&packet.RequestChunkRadius{
		ChunkRadius: 16,
	})
	if err != nil {
		return fmt.Errorf("failed to write request chunk radius packet: %v", err)
	}

	chunkRadiusUpdatedPacket, err := c.Expect(packet.IDChunkRadiusUpdated, true)
	if err != nil {
		return fmt.Errorf("failed to read chunk radius updated packet: %v", err)
	}

	_, err = c.Expect(packet.IDPlayStatus, true)
	if err != nil {
		return fmt.Errorf("failed to read play status packet: %v", err)
	}

	err = c.WritePacket(&packet.SetLocalPlayerAsInitialised{
		EntityRuntimeID: startGamePacket.(*packet.StartGame).EntityRuntimeID,
	})
	if err != nil {
		return fmt.Errorf("failed to write set local player as initialised packet: %v", err)
	}

	startGame := startGamePacket.(*packet.StartGame)
	chunkRadiusUpdated := chunkRadiusUpdatedPacket.(*packet.ChunkRadiusUpdated)

	for _, item := range startGame.Items {
		if item.Name == "minecraft:shield" {
			c.SetShieldID(int32(item.RuntimeID))
			break
		}
	}

	c.gameData = minecraft.GameData{
		WorldName:  startGame.WorldName,
		WorldSeed:  startGame.WorldSeed,
		Difficulty: startGame.Difficulty,

		EntityUniqueID:  computeEntityID(identityData.XUID),
		EntityRuntimeID: uint64(computeEntityID(identityData.XUID)),

		PlayerGameMode: startGame.PlayerGameMode,

		PersonaDisabled:     startGame.PersonaDisabled,
		CustomSkinsDisabled: startGame.CustomSkinsDisabled,

		BaseGameVersion: startGame.BaseGameVersion,

		PlayerPosition: startGame.PlayerPosition,
		Pitch:          startGame.Pitch,
		Yaw:            startGame.Yaw,

		Dimension: startGame.Dimension,

		WorldSpawn: startGame.WorldSpawn,

		EditorWorldType:    startGame.EditorWorldType,
		CreatedInEditor:    startGame.CreatedInEditor,
		ExportedFromEditor: startGame.ExportedFromEditor,

		WorldGameMode: startGame.WorldGameMode,

		GameRules: startGame.GameRules,

		Time: startGame.Time,

		ServerBlockStateChecksum: startGame.ServerBlockStateChecksum,
		CustomBlocks:             startGame.Blocks,

		Items: startGame.Items,

		PlayerMovementSettings:       startGame.PlayerMovementSettings,
		ServerAuthoritativeInventory: startGame.ServerAuthoritativeInventory,

		Experiments: startGame.Experiments,

		PlayerPermissions: startGame.PlayerPermissions,

		ChunkRadius: chunkRadiusUpdated.ChunkRadius,

		ClientSideGeneration: startGame.ClientSideGeneration,

		ChatRestrictionLevel: startGame.ChatRestrictionLevel,

		DisablePlayerInteractions: startGame.DisablePlayerInteractions,

		UseBlockNetworkIDHashes: startGame.UseBlockNetworkIDHashes,
	}
	return nil
}

// computeEntityID generates a deterministic entity ID from an XUID using FNV-1a.
func computeEntityID(xuid string) int64 {
	hash := int64(0)
	for _, char := range xuid {
		hash ^= int64(char)
		hash *= (2 ^ 40) + (2 ^ 8) + 0xb3
	}
	return hash & 0x7FFFFFFFFFFFFFFF
}
