package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"

	"github.com/cooldogedev/spectrum/protocol"
	spectrumpacket "github.com/cooldogedev/spectrum/server/packet"
	"github.com/golang/snappy"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	packetDecodeNeeded = byte(iota)
	packetDecodeNotNeeded
)

var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 256))
	},
}

var headerPool = sync.Pool{
	New: func() any {
		return &packet.Header{}
	},
}

// Conn represents a connection to a server, managing packet reading and writing
// over an underlying io.ReadWriteCloser.
type Conn struct {
	cancelFunc context.CancelCauseFunc
	ctx        context.Context

	conn   io.ReadWriteCloser
	client *minecraft.Conn
	logger *slog.Logger

	reader *protocol.Reader
	writer *protocol.Writer

	runtimeID uint64
	uniqueID  int64

	syncProtocol bool
	cache        []byte

	gameData minecraft.GameData
	shieldID int32

	protocol minecraft.Protocol
	pool     packet.Pool

	deferredPackets []any
	expectedIds     []uint32

	onConnect func(err error)

	connected chan struct{}
	spawned   chan struct{}
	once      sync.Once
}

// NewConn creates a new Conn instance using the provided io.ReadWriteCloser.
// It is used for reading and writing packets to the underlying connection.
func NewConn(conn io.ReadWriteCloser, client *minecraft.Conn, logger *slog.Logger, syncProtocol bool, cache []byte) *Conn {
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
		cache:        cache,

		protocol: proto,
		pool:     proto.Packets(false),

		connected: make(chan struct{}),
		spawned:   make(chan struct{}),
	}
	c.ctx, c.cancelFunc = context.WithCancelCause(context.Background())
	c.expect(spectrumpacket.IDConnectionResponse)
	return c
}

// ReadPacket reads the next available packet from the connection. If there are deferred packets, it will return
// one of those first. This method should not be called concurrently from multiple goroutines.
func (c *Conn) ReadPacket() (any, error) {
	select {
	case <-c.ctx.Done():
		return nil, context.Cause(c.ctx)
	case <-c.spawned:
		if len(c.deferredPackets) > 0 {
			pk := c.deferredPackets[0]
			c.deferredPackets[0] = nil
			c.deferredPackets = c.deferredPackets[1:]
			return pk, nil
		}
		return c.read()
	default:
	}

	p, err := c.read()
	if err != nil {
		return nil, err
	}

	if pk, ok := p.(packet.Packet); ok {
		if err := c.handlePacket(pk); err != nil {
			return nil, fmt.Errorf("failed to handle packet %v: %w", pk.ID(), err)
		}
	} else {
		c.deferPacket(p)
	}
	return c.ReadPacket()
}

// WritePacket encodes and writes the provided packet to the underlying connection.
func (c *Conn) WritePacket(pk packet.Packet) error {
	select {
	case <-c.ctx.Done():
		return context.Cause(c.ctx)
	default:
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	header := headerPool.Get().(*packet.Header)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
		headerPool.Put(header)
	}()

	header.PacketID = pk.ID()
	if err := header.Write(buf); err != nil {
		return err
	}
	pk.Marshal(c.protocol.NewWriter(buf, c.shieldID))
	return c.writer.Write(snappy.Encode(nil, buf.Bytes()))
}

// Write writes provided byte slice to the underlying connection.
func (c *Conn) Write(p []byte) error {
	return c.writer.Write(snappy.Encode(nil, p))
}

// DoConnect sends a ConnectionRequest packet to initiate the connection sequence.
func (c *Conn) DoConnect() error {
	select {
	case <-c.ctx.Done():
		return context.Cause(c.ctx)
	default:
	}

	clientData, err := json.Marshal(c.client.ClientData())
	if err != nil {
		return err
	}

	identityData, err := json.Marshal(c.client.IdentityData())
	if err != nil {
		return err
	}

	err = c.WritePacket(&spectrumpacket.ConnectionRequest{
		Addr:         c.client.RemoteAddr().String(),
		ProtocolID:   c.protocol.ID(),
		ClientData:   clientData,
		IdentityData: identityData,
		Cache:        c.cache,
	})
	if err != nil {
		return err
	}
	c.logger.Debug("sent connection_request, expecting connection_response")
	return nil
}

// OnConnect invokes the provided function once the connection sequence is complete or has failed.
func (c *Conn) OnConnect(fn func(error)) {
	c.onConnect = fn
}

// WaitConnect blocks until the connection sequence has completed or the provided context is canceled.
func (c *Conn) WaitConnect(ctx context.Context) error {
	select {
	case <-c.ctx.Done():
		return context.Cause(c.ctx)
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-c.connected:
		return nil
	}
}

// DoSpawn sends a SetLocalPlayerAsInitialised packet to spawn the player in the server
// and signals that packets can now be read.
func (c *Conn) DoSpawn() error {
	select {
	case <-c.ctx.Done():
		return context.Cause(c.ctx)
	default:
	}
	close(c.spawned)
	return c.WritePacket(&packet.SetLocalPlayerAsInitialised{EntityRuntimeID: c.runtimeID})
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
		var connected bool
		select {
		case <-c.connected:
			connected = true
		default:
		}

		if !connected && c.onConnect != nil {
			c.onConnect(err)
		}
		c.cancelFunc(err)
		_ = c.conn.Close()
	})
}

// read reads a packet from the connection, handling decompression and decoding as necessary.
// Packets are prefixed with a special byte (packetDecodeNeeded or packetDecodeNotNeeded) indicating
// the decoding necessity. If decode is false and the packet does not require decoding,
// it returns the raw decompressed payload.
func (c *Conn) read() (pk any, err error) {
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
	header := headerPool.Get().(*packet.Header)
	defer func() {
		headerPool.Put(header)
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet %v: %v", header.PacketID, r)
		}
	}()
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

// deferPacket defers a packet to be returned later in ReadPacket().
func (c *Conn) deferPacket(pk any) {
	c.deferredPackets = append(c.deferredPackets, pk)
}

// expect stores packet IDs that will be read and handled before finalizing the connection sequence.
func (c *Conn) expect(ids ...uint32) {
	c.expectedIds = ids
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
		if !slices.Contains(c.expectedIds, pk.ID()) {
			c.deferPacket(pk)
			continue
		}

		switch pk := pk.(type) {
		case *spectrumpacket.ConnectionResponse:
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
func (c *Conn) handleConnectionResponse(pk *spectrumpacket.ConnectionResponse) error {
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
	close(c.connected)
	if c.onConnect != nil {
		c.onConnect(nil)
	}
	c.logger.Debug("received play_status, finalizing connection sequence")
	return nil
}
