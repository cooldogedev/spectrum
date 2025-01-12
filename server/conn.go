package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cooldogedev/spectrum/internal"
	"github.com/cooldogedev/spectrum/protocol"
	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/golang/snappy"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	packetDecodeNeeded    = 0x00
	packetDecodeNotNeeded = 0x01
)

// Conn represents a connection to a server, managing packet reading and writing
// over an underlying io.ReadWriteCloser.
type Conn struct {
	conn       io.ReadWriteCloser
	clientConn *minecraft.Conn
	logger     *slog.Logger

	reader   *protocol.Reader
	writer   *protocol.Writer
	writerMu sync.Mutex

	runtimeID uint64
	uniqueID  int64
	token     string

	gameData minecraft.GameData
	shieldID int32

	protocol minecraft.Protocol
	pool     packet.Pool

	deferredPackets []any
	expectedIds     atomic.Value
	header          *packet.Header

	connected chan struct{}
	spawned   chan struct{}
	closed    chan struct{}
}

// NewConn creates a new Conn instance using the provided io.ReadWriteCloser.
// It is used for reading and writing packets to the underlying connection.
func NewConn(conn io.ReadWriteCloser, client *minecraft.Conn, logger *slog.Logger, proto minecraft.Protocol, token string) *Conn {
	c := &Conn{
		conn:       conn,
		clientConn: client,
		logger:     logger,

		reader: protocol.NewReader(conn),
		writer: protocol.NewWriter(conn),

		token: token,

		protocol: proto,
		pool:     proto.Packets(false),
		header:   &packet.Header{},

		connected: make(chan struct{}),
		spawned:   make(chan struct{}),
		closed:    make(chan struct{}),
	}
	go func() {
		for {
			select {
			case <-c.closed:
				return
			case <-c.spawned:
				return
			default:
				payload, err := c.read()
				if err != nil {
					_ = c.Close()
					c.logger.Error("failed to read connection sequence packet", "err", err)
					return
				}

				pk, ok := payload.(packet.Packet)
				if !ok {
					c.deferPacket(payload)
					continue
				}

				handled := false
				for _, id := range c.expectedIds.Load().([]uint32) {
					for _, latest := range c.protocol.ConvertToLatest(pk, client) {
						if pk.ID() != id {
							continue
						}

						handled = true
						deferrable, err := c.handlePacket(latest)
						if err != nil {
							c.logger.Error("failed to handle connection sequence packet", "err", err)
							_ = c.Close()
							return
						}

						if deferrable {
							c.deferPacket(pk)
						}
					}
				}

				if !handled {
					c.deferPacket(pk)
				}
			}
		}
	}()
	return c
}

// ReadPacket reads the next available packet from the connection. If there are deferred packets, it will return
// one of those first. This method should not be called concurrently from multiple goroutines.
func (c *Conn) ReadPacket() (any, error) {
	if len(c.deferredPackets) > 0 {
		pk := c.deferredPackets[0]
		c.deferredPackets[0] = nil
		c.deferredPackets = c.deferredPackets[1:]
		return pk, nil
	}
	return c.read()
}

// WritePacket encodes and writes the provided packet to the underlying connection.
func (c *Conn) WritePacket(pk packet.Packet) error {
	c.writerMu.Lock()
	defer c.writerMu.Unlock()

	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		internal.BufferPool.Put(buf)
	}()

	c.header.PacketID = pk.ID()
	if err := c.header.Write(buf); err != nil {
		return err
	}
	pk.Marshal(c.protocol.NewWriter(buf, c.shieldID))
	return c.writer.Write(snappy.Encode(nil, buf.Bytes()))
}

// Write writes provided byte slice to the underlying connection.
func (c *Conn) Write(p []byte) error {
	c.writerMu.Lock()
	defer c.writerMu.Unlock()
	return c.writer.Write(snappy.Encode(nil, p))
}

// Connect initiates the connection sequence with a default timeout of 1 minute.
func (c *Conn) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return c.ConnectContext(ctx)
}

// ConnectTimeout initiates the connection sequence with the specified timeout duration.
func (c *Conn) ConnectTimeout(duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	return c.ConnectContext(ctx)
}

// ConnectContext initiates the connection sequence using the provided context for cancellation.
func (c *Conn) ConnectContext(ctx context.Context) error {
	c.expect(packet2.IDConnectionResponse)
	if err := c.sendConnectionRequest(); err != nil {
		return err
	}

	select {
	case <-c.closed:
		return net.ErrClosed
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-c.connected:
		return nil
	}
}

// Spawn initiates the spawning sequence with a default timeout of 1 minute.
func (c *Conn) Spawn() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return c.SpawnContext(ctx)
}

// SpawnTimeout initiates the spawning sequence with the specified timeout duration.
func (c *Conn) SpawnTimeout(duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	return c.SpawnContext(ctx)
}

// SpawnContext initiates the spawning sequence using the provided context for cancellation.
func (c *Conn) SpawnContext(ctx context.Context) error {
	c.expect(packet.IDStartGame)
	select {
	case <-c.closed:
		return net.ErrClosed
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-c.spawned:
		return nil
	}
}

// Conn returns the underlying connection.
// Direct access to the underlying connection through this method is
// strongly discouraged due to the potential for unpredictable behavior.
// Use this method only when absolutely necessary.
func (c *Conn) Conn() io.ReadWriteCloser {
	return c.conn
}

// GameData returns the game data set for the connection by the StartGame packet.
func (c *Conn) GameData() minecraft.GameData {
	return c.gameData
}

// ShieldID returns the shield id set for the connection by the StartGame packet.
func (c *Conn) ShieldID() int32 {
	return c.shieldID
}

// Close closes the underlying connection.
func (c *Conn) Close() (err error) {
	select {
	case <-c.closed:
		return errors.New("already closed")
	default:
		close(c.closed)
		_ = c.conn.Close()
		return
	}
}

// read reads a packet from the connection, handling decompression and decoding as necessary.
// Packets are prefixed with a special byte (packetDecodeNeeded or packetDecodeNotNeeded) indicating
// the decoding necessity. If decode is false and the packet does not require decoding,
// it returns the raw decompressed payload.
func (c *Conn) read() (pk any, err error) {
	select {
	case <-c.closed:
		return nil, net.ErrClosed
	default:
		payload, err := c.reader.ReadPacket()
		if err != nil {
			return nil, err
		}

		if payload[0] != packetDecodeNeeded && payload[0] != packetDecodeNotNeeded {
			return nil, fmt.Errorf("unknown decode byte marker %v", payload[0])
		}

		decompressed, err := snappy.Decode(nil, payload[1:])
		if err != nil {
			return nil, err
		}

		if payload[0] == packetDecodeNotNeeded {
			return decompressed, nil
		}

		buf := bytes.NewBuffer(decompressed)
		header := &packet.Header{}
		if err := header.Read(buf); err != nil {
			return nil, err
		}

		factory, ok := c.pool[header.PacketID]
		if !ok {
			return nil, fmt.Errorf("unknown packet ID %v", header.PacketID)
		}
		pk = factory()
		pk.(packet.Packet).Marshal(c.protocol.NewReader(buf, c.shieldID, false))
		return pk, nil
	}
}

// deferPacket defers a packet to be returned later in ReadPacket().
func (c *Conn) deferPacket(pk any) {
	c.deferredPackets = append(c.deferredPackets, pk)
}

// expect stores packet IDs that will be read and handled before finalizing the spawning sequence.
func (c *Conn) expect(ids ...uint32) {
	c.expectedIds.Store(ids)
}

// sendConnectionRequest initiates the connection sequence by sending a ConnectionRequest packet to the underlying connection.
func (c *Conn) sendConnectionRequest() error {
	clientData, err := json.Marshal(c.clientConn.ClientData())
	if err != nil {
		return err
	}

	identityData, err := json.Marshal(c.clientConn.IdentityData())
	if err != nil {
		return err
	}

	err = c.WritePacket(&packet2.ConnectionRequest{
		Addr:         c.clientConn.RemoteAddr().String(),
		Token:        c.token,
		ClientData:   clientData,
		IdentityData: identityData,
	})
	if err != nil {
		return err
	}
	c.logger.Debug("sent connection_request, expecting connection_response")
	return nil
}

// handlePacket handles an expected packet that was received before the spawning sequence finalization.
func (c *Conn) handlePacket(pk packet.Packet) (bool, error) {
	switch pk := pk.(type) {
	case *packet2.ConnectionResponse:
		return c.handleConnectionResponse(pk)
	case *packet.StartGame:
		return c.handleStartGame(pk)
	case *packet.ChunkRadiusUpdated:
		return c.handleChunkRadiusUpdated(pk)
	case *packet.PlayStatus:
		return c.handlePlayStatus(pk)
	default:
		return false, nil
	}
}

// handleConnectionResponse handles the ConnectionResponse, which is the final packet in the connection sequence
// it signals that we may proceed with the spawning sequence.
func (c *Conn) handleConnectionResponse(pk *packet2.ConnectionResponse) (bool, error) {
	c.runtimeID = pk.RuntimeID
	c.uniqueID = pk.UniqueID
	close(c.connected)
	c.logger.Debug("received connection_response, expecting start_game")
	return false, nil
}

// handleStartGame handles the StartGame packet.
func (c *Conn) handleStartGame(pk *packet.StartGame) (bool, error) {
	c.expect(packet.IDChunkRadiusUpdated)
	c.gameData = minecraft.GameData{
		Difficulty:                   pk.Difficulty,
		WorldName:                    pk.WorldName,
		WorldSeed:                    pk.WorldSeed,
		EntityUniqueID:               c.uniqueID,
		EntityRuntimeID:              c.runtimeID,
		PlayerGameMode:               pk.PlayerGameMode,
		BaseGameVersion:              pk.BaseGameVersion,
		PlayerPosition:               pk.PlayerPosition,
		Pitch:                        pk.Pitch,
		Yaw:                          pk.Yaw,
		Dimension:                    pk.Dimension,
		WorldSpawn:                   pk.WorldSpawn,
		EditorWorldType:              pk.EditorWorldType,
		CreatedInEditor:              pk.CreatedInEditor,
		ExportedFromEditor:           pk.ExportedFromEditor,
		PersonaDisabled:              pk.PersonaDisabled,
		CustomSkinsDisabled:          pk.CustomSkinsDisabled,
		GameRules:                    pk.GameRules,
		Time:                         pk.Time,
		ServerBlockStateChecksum:     pk.ServerBlockStateChecksum,
		CustomBlocks:                 pk.Blocks,
		Items:                        pk.Items,
		PlayerMovementSettings:       pk.PlayerMovementSettings,
		WorldGameMode:                pk.WorldGameMode,
		Hardcore:                     pk.Hardcore,
		ServerAuthoritativeInventory: pk.ServerAuthoritativeInventory,
		PlayerPermissions:            pk.PlayerPermissions,
		ChatRestrictionLevel:         pk.ChatRestrictionLevel,
		DisablePlayerInteractions:    pk.DisablePlayerInteractions,
		ClientSideGeneration:         pk.ClientSideGeneration,
		Experiments:                  pk.Experiments,
		UseBlockNetworkIDHashes:      pk.UseBlockNetworkIDHashes,
	}
	for _, item := range pk.Items {
		if item.Name == "minecraft:shield" {
			c.shieldID = int32(item.RuntimeID)
		}
	}

	if err := c.WritePacket(&packet.RequestChunkRadius{ChunkRadius: 16}); err != nil {
		return false, err
	}
	c.logger.Debug("received start_game, expecting chunk_radius_updated")
	return false, nil
}

// handleChunkRadiusUpdated handles the first ChunkRadiusUpdated packet, which updates the initial chunk
// radius of the connection.
func (c *Conn) handleChunkRadiusUpdated(pk *packet.ChunkRadiusUpdated) (bool, error) {
	c.expect(packet.IDPlayStatus)
	c.gameData.ChunkRadius = pk.ChunkRadius
	c.logger.Debug("received chunk_radius_updated, expecting play_status")
	return true, nil
}

// handlePlayStatus handles the first PlayStatus packet. It is the final packet in the spawning sequence,
// it responds to the server with a packet.SetLocalPlayerAsInitialised to finalize the spawning sequence and spawn the player.
func (c *Conn) handlePlayStatus(_ *packet.PlayStatus) (bool, error) {
	if err := c.WritePacket(&packet.SetLocalPlayerAsInitialised{EntityRuntimeID: c.runtimeID}); err != nil {
		return false, err
	}
	close(c.spawned)
	c.logger.Debug("received play_status, finalizing spawn sequence")
	return true, nil
}
