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
	"slices"
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
	cancelFunc context.CancelCauseFunc
	ctx        context.Context

	conn   io.ReadWriteCloser
	client *minecraft.Conn
	logger *slog.Logger

	reader   *protocol.Reader
	writer   *protocol.Writer
	writerMu sync.Mutex

	runtimeID uint64
	uniqueID  int64

	syncProtocol bool
	token        string

	gameData minecraft.GameData
	shieldID int32

	protocol minecraft.Protocol
	pool     packet.Pool

	deferredPackets []any
	expectedIds     atomic.Value
	header          *packet.Header

	connected chan struct{}
	once      sync.Once
}

// NewConn creates a new Conn instance using the provided io.ReadWriteCloser.
// It is used for reading and writing packets to the underlying connection.
func NewConn(conn io.ReadWriteCloser, client *minecraft.Conn, logger *slog.Logger, syncProtocol bool, token string) *Conn {
	var proto minecraft.Protocol
	if syncProtocol {
		proto = client.Proto()
	} else {
		proto = minecraft.DefaultProtocol
	}

	c := &Conn{
		conn:   conn,
		client: client,
		logger: logger,

		reader: protocol.NewReader(conn),
		writer: protocol.NewWriter(conn),

		syncProtocol: syncProtocol,
		token:        token,

		protocol: proto,
		pool:     proto.Packets(false),
		header:   &packet.Header{},

		connected: make(chan struct{}),
	}
	c.ctx, c.cancelFunc = context.WithCancelCause(client.Context())
	go func() {
	read:
		for {
			select {
			case <-c.ctx.Done():
				break read
			case <-c.connected:
				break read
			default:
				payload, err := c.read()
				if err != nil {
					c.CloseWithError(fmt.Errorf("failed to read connection sequence packet: %w", err))
					c.logger.Error("failed to read connection sequence packet", "err", err)
					break read
				}

				pk, ok := payload.(packet.Packet)
				if !ok {
					c.deferPacket(payload)
					continue
				}

				if err := c.handlePacket(pk); err != nil {
					c.CloseWithError(fmt.Errorf("failed to handle connection sequence packet: %w", err))
					c.logger.Error("failed to handle connection sequence packet", "err", err)
					break read
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
	case <-c.ctx.Done():
		return net.ErrClosed
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-c.connected:
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

// Context returns the connection's context. The context is canceled when the connection is closed,
// allowing for cancellation of operations that are tied to the lifecycle of the connection.
func (c *Conn) Context() context.Context {
	return c.ctx
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	c.CloseWithError(errors.New("closed by application"))
	return nil
}

// CloseWithError closes the underlying connection.
func (c *Conn) CloseWithError(err error) {
	c.once.Do(func() {
		c.cancelFunc(err)
		_ = c.conn.Close()
	})
}

// read reads a packet from the connection, handling decompression and decoding as necessary.
// Packets are prefixed with a special byte (packetDecodeNeeded or packetDecodeNotNeeded) indicating
// the decoding necessity. If decode is false and the packet does not require decoding,
// it returns the raw decompressed payload.
func (c *Conn) read() (pk any, err error) {
	select {
	case <-c.ctx.Done():
		return nil, net.ErrClosed
	default:
	}

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

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet %v: %v", header.PacketID, r)
		}
	}()
	factory, ok := c.pool[header.PacketID]
	if !ok {
		return nil, fmt.Errorf("unknown packet ID %v", header.PacketID)
	}
	pk = factory()
	pk.(packet.Packet).Marshal(c.protocol.NewReader(buf, c.shieldID, false))
	return pk, nil
}

// deferPacket defers a packet to be returned later in ReadPacket().
func (c *Conn) deferPacket(pk any) {
	c.deferredPackets = append(c.deferredPackets, pk)
}

// expect stores packet IDs that will be read and handled before finalizing the connection sequence.
func (c *Conn) expect(ids ...uint32) {
	c.expectedIds.Store(ids)
}

// sendConnectionRequest initiates the connection sequence by sending a ConnectionRequest packet to the underlying connection.
func (c *Conn) sendConnectionRequest() error {
	clientData, err := json.Marshal(c.client.ClientData())
	if err != nil {
		return err
	}

	identityData, err := json.Marshal(c.client.IdentityData())
	if err != nil {
		return err
	}

	err = c.WritePacket(&packet2.ConnectionRequest{
		Addr:         c.client.RemoteAddr().String(),
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

// handlePacket handles an expected packet that was received before the connection sequence finalization.
func (c *Conn) handlePacket(p packet.Packet) (err error) {
	var pks []packet.Packet
	if c.syncProtocol {
		pks = c.protocol.ConvertToLatest(p, c.client)
	} else {
		pks = []packet.Packet{p}
	}

	for _, pk := range pks {
		if !slices.Contains(c.expectedIds.Load().([]uint32), pk.ID()) {
			c.deferPacket(pk)
			continue
		}

		switch pk := pk.(type) {
		case *packet2.ConnectionResponse:
			err = c.handleConnectionResponse(pk)
		case *packet.StartGame:
			err = c.handleStartGame(pk)
		case *packet.ItemRegistry:
			err = c.handleItemRegistry(pk)
		case *packet.ChunkRadiusUpdated:
			err = c.handleChunkRadiusUpdated(pk)
		case *packet.PlayStatus:
			err = c.handlePlayStatus(pk)
		default:
			c.deferPacket(pk)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

// handleConnectionResponse handles the ConnectionResponse packet.
func (c *Conn) handleConnectionResponse(pk *packet2.ConnectionResponse) error {
	c.expect(packet.IDStartGame)
	c.runtimeID = pk.RuntimeID
	c.uniqueID = pk.UniqueID
	c.logger.Debug("received connection_response, expecting start_game")
	return nil
}

// handleStartGame handles the StartGame packet.
func (c *Conn) handleStartGame(pk *packet.StartGame) error {
	c.expect(packet.IDItemRegistry)
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
	c.logger.Debug("received start_game, expecting item_registry")
	return nil
}

// handleItemRegistry handles the ItemRegistry packet.
func (c *Conn) handleItemRegistry(pk *packet.ItemRegistry) error {
	c.deferPacket(pk)
	c.expect(packet.IDChunkRadiusUpdated)
	c.gameData.Items = pk.Items
	for _, item := range pk.Items {
		if item.Name == "minecraft:shield" {
			c.shieldID = int32(item.RuntimeID)
		}
	}

	if err := c.WritePacket(&packet.RequestChunkRadius{ChunkRadius: 16}); err != nil {
		return err
	}
	c.logger.Debug("received item_registry, expecting chunk_radius_updated")
	return nil
}

// handleChunkRadiusUpdated handles the first ChunkRadiusUpdated packet, which updates the initial chunk
// radius of the connection.
func (c *Conn) handleChunkRadiusUpdated(pk *packet.ChunkRadiusUpdated) error {
	c.deferPacket(pk)
	c.expect(packet.IDPlayStatus)
	c.gameData.ChunkRadius = pk.ChunkRadius
	c.logger.Debug("received chunk_radius_updated, expecting play_status")
	return nil
}

// handlePlayStatus handles the first PlayStatus packet. It is the final packet in the connection sequence,
// it responds to the server with a packet.SetLocalPlayerAsInitialised to finalize the connection sequence and spawn the player.
func (c *Conn) handlePlayStatus(pk *packet.PlayStatus) error {
	c.deferPacket(pk)
	if err := c.WritePacket(&packet.SetLocalPlayerAsInitialised{EntityRuntimeID: c.runtimeID}); err != nil {
		return err
	}
	close(c.connected)
	c.logger.Debug("received play_status, finalizing connection sequence")
	return nil
}
