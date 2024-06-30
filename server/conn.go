package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/cooldogedev/spectrum/internal"
	proto "github.com/cooldogedev/spectrum/protocol"
	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/golang/snappy"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	packetDecodeNeeded    = 0x00
	packetDecodeNotNeeded = 0x01
)

const shieldRuntimeID = "minecraft:shield"

// Conn is a connection to a server. It is used to read and write packets to the server, and to manage the
// connection to the server.
type Conn struct {
	conn   io.ReadWriteCloser
	reader *proto.Reader
	writer *proto.Writer

	runtimeID uint64
	uniqueID  int64

	gameData minecraft.GameData
	shieldID int32

	deferredPackets []packet.Packet
	header          packet.Header
	pool            packet.Pool

	ch chan struct{}
	mu sync.Mutex
}

// NewConn creates a new Conn with the conn and pool passed.
func NewConn(conn io.ReadWriteCloser, pool packet.Pool) *Conn {
	return &Conn{
		conn:   conn,
		reader: proto.NewReader(conn),
		writer: proto.NewWriter(conn),

		header: packet.Header{},
		pool:   pool,

		ch: make(chan struct{}),
	}
}

// ReadPacket reads a packet from the connection. It returns the packet read, or an error if the packet could not
// be read.
func (c *Conn) ReadPacket() (any, error) {
	if len(c.deferredPackets) > 0 {
		pk := c.deferredPackets[0]
		c.deferredPackets[0] = nil
		c.deferredPackets = c.deferredPackets[1:]
		return pk, nil
	}
	return c.read(false)
}

// WritePacket writes a packet to the connection. It returns an error if the packet could not be written.
func (c *Conn) WritePacket(pk packet.Packet) error {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	return c.writer.Write(snappy.Encode(nil, buf.Bytes()))
}

// Connect send a connection request to the server with the client and token passed. It returns
// an error if the server doesn't respond.
func (c *Conn) Connect(client *minecraft.Conn, token string) (err error) {
	clientDataBytes, _ := json.Marshal(client.ClientData())
	identityDataBytes, _ := json.Marshal(client.IdentityData())

	err = c.WritePacket(&packet2.ConnectionRequest{
		Addr:         client.RemoteAddr().String(),
		Token:        token,
		ClientData:   clientDataBytes,
		IdentityData: identityDataBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to write connection request packet: %v", err)
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

// Spawn will start the spawning sequence.
func (c *Conn) Spawn() (err error) {
	startGamePacket, err := c.expect(packet.IDStartGame, false)
	if err != nil {
		return fmt.Errorf("failed to read start game packet: %v", err)
	}

	if err := c.WritePacket(&packet.RequestChunkRadius{ChunkRadius: 16}); err != nil {
		return fmt.Errorf("failed to write request chunk radius packet: %v", err)
	}

	chunkRadiusUpdatedPacket, err := c.expect(packet.IDChunkRadiusUpdated, true)
	if err != nil {
		return fmt.Errorf("failed to read chunk radius updated packet: %v", err)
	}

	if _, err := c.expect(packet.IDPlayStatus, true); err != nil {
		return fmt.Errorf("failed to read play status packet: %v", err)
	}

	if err := c.WritePacket(&packet.SetLocalPlayerAsInitialised{EntityRuntimeID: c.runtimeID}); err != nil {
		return fmt.Errorf("failed to write set local player as initialised packet: %v", err)
	}

	startGame := startGamePacket.(*packet.StartGame)
	for _, item := range startGame.Items {
		if item.Name == shieldRuntimeID {
			c.shieldID = int32(item.RuntimeID)
			break
		}
	}
	c.gameData = minecraft.GameData{
		WorldName:                    startGame.WorldName,
		WorldSeed:                    startGame.WorldSeed,
		Difficulty:                   startGame.Difficulty,
		EntityUniqueID:               c.uniqueID,
		EntityRuntimeID:              c.runtimeID,
		PlayerGameMode:               startGame.PlayerGameMode,
		PersonaDisabled:              startGame.PersonaDisabled,
		CustomSkinsDisabled:          startGame.CustomSkinsDisabled,
		BaseGameVersion:              startGame.BaseGameVersion,
		PlayerPosition:               startGame.PlayerPosition,
		Pitch:                        startGame.Pitch,
		Yaw:                          startGame.Yaw,
		Dimension:                    startGame.Dimension,
		WorldSpawn:                   startGame.WorldSpawn,
		EditorWorldType:              startGame.EditorWorldType,
		CreatedInEditor:              startGame.CreatedInEditor,
		ExportedFromEditor:           startGame.ExportedFromEditor,
		WorldGameMode:                startGame.WorldGameMode,
		GameRules:                    startGame.GameRules,
		Time:                         startGame.Time,
		ServerBlockStateChecksum:     startGame.ServerBlockStateChecksum,
		CustomBlocks:                 startGame.Blocks,
		Items:                        startGame.Items,
		PlayerMovementSettings:       startGame.PlayerMovementSettings,
		ServerAuthoritativeInventory: startGame.ServerAuthoritativeInventory,
		Experiments:                  startGame.Experiments,
		PlayerPermissions:            startGame.PlayerPermissions,
		ChunkRadius:                  chunkRadiusUpdatedPacket.(*packet.ChunkRadiusUpdated).ChunkRadius,
		ClientSideGeneration:         startGame.ClientSideGeneration,
		ChatRestrictionLevel:         startGame.ChatRestrictionLevel,
		DisablePlayerInteractions:    startGame.DisablePlayerInteractions,
		UseBlockNetworkIDHashes:      startGame.UseBlockNetworkIDHashes,
	}
	return
}

// GameData ...
func (c *Conn) GameData() minecraft.GameData {
	return c.gameData
}

// Close ...
func (c *Conn) Close() (err error) {
	select {
	case <-c.ch:
		return errors.New("already closed")
	default:
		close(c.ch)
		_ = c.conn.Close()
		return
	}
}

// read reads a packet from the connection. It returns the packet read, or an error if the packet could not
// be read.
func (c *Conn) read(decode bool) (any, error) {
	select {
	case <-c.ch:
		return nil, errors.New("closed connection")
	default:
		payload, err := c.reader.ReadPacket()
		if err != nil {
			return nil, err
		}

		decompressed, err := snappy.Decode(nil, payload[1:])
		if err != nil {
			return nil, err
		}

		if payload[0] == packetDecodeNeeded || decode {
			return c.decode(decompressed)
		} else if payload[0] == packetDecodeNotNeeded {
			return decompressed, nil
		}
		return nil, fmt.Errorf("received unknown expect marker byte %v", payload[0])
	}
}

// decode decodes a packet payload and returns the decoded packet or an error if the packet could not be decoded.
func (c *Conn) decode(payload []byte) (pk packet.Packet, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while reading packet: %v", r)
		}
	}()

	buf := bytes.NewBuffer(payload)
	header := &packet.Header{}
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
func (c *Conn) expect(id uint32, deferrable bool) (packet.Packet, error) {
	payload, err := c.read(true)
	if err != nil {
		return nil, err
	}

	pk := payload.(packet.Packet)
	if pk.ID() != id || deferrable {
		c.deferredPackets = append(c.deferredPackets, pk)
	}

	if pk.ID() == id {
		return pk, nil
	}
	return c.expect(id, deferrable)
}
