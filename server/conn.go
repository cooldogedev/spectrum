package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cooldogedev/spectrum/internal"
	proto "github.com/cooldogedev/spectrum/protocol"
	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"net"
	"sync"
)

const (
	packetDecodeNeeded    = 0x00
	packetDecodeNotNeeded = 0x01
)

// Conn is a connection to a server. It is used to read and write packets to the server, and to manage the
// connection to the server.
type Conn struct {
	conn       net.Conn
	compressor packet.Compression

	reader *proto.Reader

	writer  *proto.Writer
	writeMu sync.Mutex

	gameData  minecraft.GameData
	runtimeID uint64
	uniqueID  int64
	shieldID  int32

	pool            packet.Pool
	header          packet.Header
	deferredPackets []packet.Packet

	closed chan struct{}
}

// NewConn creates a new Conn with the conn and pool passed.
func NewConn(conn net.Conn, pool packet.Pool) *Conn {
	c := &Conn{
		conn:       conn,
		compressor: packet.FlateCompression,

		reader: proto.NewReader(conn),
		writer: proto.NewWriter(conn),

		pool:   pool,
		header: packet.Header{},

		closed: make(chan struct{}),
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
func (c *Conn) ReadPacket(decode bool) (any, error) {
	select {
	case <-c.closed:
		return nil, fmt.Errorf("connection closed")
	default:
		payload := c.reader.ReadPacket()
		decompressed, err := c.compressor.Decompress(payload[1:])
		if err != nil {
			return nil, err
		}

		if payload[0] == packetDecodeNeeded || decode {
			return c.decode(decompressed)
		} else if payload[0] == packetDecodeNotNeeded {
			return decompressed, nil
		}
		return nil, fmt.Errorf("received unknown decode marker byte %v", payload[0])
	}
}

// ReadDeferred reads all packets deferred in the connection and returns them. It returns an empty slice if no
// packets were deferred.
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

	pk.Marshal(protocol.NewWriter(buf, c.shieldID))
	data, err := c.compressor.Compress(buf.Bytes())
	if err != nil {
		return err
	}
	return c.writer.Write(data)
}

// Spawn will start the spawning sequence using the game data found in conn.GameData(), which was
// sent earlier by the server.
func (c *Conn) Spawn() (err error) {
	startGamePacket, err := c.expect(packet.IDStartGame, false)
	if err != nil {
		return fmt.Errorf("failed to read start game packet: %v", err)
	}

	err = c.WritePacket(&packet.RequestChunkRadius{ChunkRadius: 16})
	if err != nil {
		return fmt.Errorf("failed to write request chunk radius packet: %v", err)
	}

	chunkRadiusUpdatedPacket, err := c.expect(packet.IDChunkRadiusUpdated, true)
	if err != nil {
		return fmt.Errorf("failed to read chunk radius updated packet: %v", err)
	}

	_, err = c.expect(packet.IDPlayStatus, true)
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
			c.shieldID = int32(item.RuntimeID)
			break
		}
	}

	c.gameData = minecraft.GameData{
		WorldName:  startGame.WorldName,
		WorldSeed:  startGame.WorldSeed,
		Difficulty: startGame.Difficulty,

		EntityUniqueID:  c.uniqueID,
		EntityRuntimeID: c.runtimeID,

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
	return
}

// GameData ...
func (c *Conn) GameData() minecraft.GameData {
	return c.gameData
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
		_ = c.conn.Close()
	}
}

// decode decodes a packet payload and returns the decoded packet or an error if the packet could not be decoded.
func (c *Conn) decode(payload []byte) (pk packet.Packet, err error) {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Write(payload)
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
	pk.Marshal(protocol.NewReader(buf, c.shieldID, false))
	return pk, nil
}

// expect reads a packet from the connection and expects it to have the ID passed. If the packet read does not
// have the ID passed, it will be deferred and the function will be called again until a packet with the ID
// passed is read. It returns the packet read, or an error if the packet could not be read.
func (c *Conn) expect(id uint32, deferrable bool) (pk packet.Packet, err error) {
	payload, err := c.ReadPacket(true)
	if err != nil {
		return nil, err
	}

	pk, _ = payload.(packet.Packet)
	if pk.ID() != id {
		c.deferredPackets = append(c.deferredPackets, pk)
		return c.expect(id, deferrable)
	}

	if deferrable {
		c.deferredPackets = append(c.deferredPackets, pk)
	}
	return
}

// connect send a connection request to the server with the address, clientData and identityData passed. It returns
// an error if the server doesn't respond.
func (c *Conn) connect(addr string, clientData login.ClientData, identityData login.IdentityData) (err error) {
	clientDataBytes, _ := json.Marshal(clientData)
	identityDataBytes, _ := json.Marshal(identityData)

	err = c.WritePacket(&packet2.ConnectionRequest{
		Addr:         addr,
		ClientData:   clientDataBytes,
		IdentityData: identityDataBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to write connect packet: %v", err)
	}

	connectionResponsePacket, err := c.expect(packet2.IDConnectionResponse, false)
	if err != nil {
		return fmt.Errorf("failed to read connection response packet: %v", err)
	}

	connectionResponse, _ := connectionResponsePacket.(*packet2.ConnectionResponse)
	c.runtimeID = connectionResponse.RuntimeID
	c.uniqueID = connectionResponse.UniqueID
	return
}
