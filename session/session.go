package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session/animation"
	"github.com/cooldogedev/spectrum/transport"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Session represents a player session within the proxy, managing client and server interactions,
// including transfers, fallbacks, and tracking various session states.
type Session struct {
	ctx        context.Context
	cancelFunc context.CancelCauseFunc

	client *minecraft.Conn

	serverAddr string
	serverConn *server.Conn
	serverMu   sync.RWMutex

	logger   *slog.Logger
	registry *Registry

	discovery server.Discovery
	opts      util.Opts
	transport transport.Transport

	animation animation.Animation
	tracker   *tracker

	processor   Processor
	processorMu sync.RWMutex

	cache             atomic.Value
	latency           atomic.Int64
	transferring      atomic.Bool
	fallbackInProcess atomic.Bool
	once              sync.Once
}

// NewSession creates a new Session instance using the provided minecraft.Conn.
func NewSession(client *minecraft.Conn, logger *slog.Logger, registry *Registry, discovery server.Discovery, opts util.Opts, transport transport.Transport) *Session {
	s := &Session{
		client: client,

		logger:   logger,
		registry: registry,

		discovery: discovery,
		opts:      opts,
		transport: transport,

		processor: NopProcessor{},

		animation: &animation.Dimension{},
		tracker:   newTracker(),
	}
	s.ctx, s.cancelFunc = context.WithCancelCause(client.Context())
	s.cache.Store([]byte(nil))
	return s
}

// Login initiates the login sequence with a default timeout of 1 minute.
func (s *Session) Login() (err error) {
	ctx, cancel := context.WithTimeout(s.ctx, time.Minute)
	defer cancel()
	return s.LoginContext(ctx)
}

// LoginTimeout initiates the login sequence with the specified timeout duration.
func (s *Session) LoginTimeout(duration time.Duration) (err error) {
	ctx, cancel := context.WithTimeout(s.ctx, duration)
	defer cancel()
	return s.LoginContext(ctx)
}

// LoginContext initiates the login sequence for the session, including server discovery,
// establishing a connection, and spawning the player in the game. The process is performed
// using the provided context for cancellation.
func (s *Session) LoginContext(ctx context.Context) (err error) {
	identityData := s.client.IdentityData()
	serverAddr, err := s.discovery.Discover(s.client)
	if err != nil {
		s.logger.Debug("discovery failed", "err", err)
		return err
	}

	conn, err := s.dial(ctx, serverAddr)
	if err != nil {
		s.logger.Debug("dialer failed", "err", err)
		return err
	}

	s.serverMu.Lock()
	s.serverAddr = serverAddr
	s.serverConn = conn
	s.serverMu.Unlock()
	go handleServer(s)
	go handleClient(s)
	go handleLatency(s, s.opts.LatencyInterval)
	if err := conn.ConnectContext(ctx); err != nil {
		s.logger.Debug("connection sequence failed", "err", err)
		return err
	}

	gameData := conn.GameData()
	s.Processor().ProcessStartGame(NewContext(), &gameData)
	if err := s.client.StartGame(gameData); err != nil {
		s.logger.Debug("startgame sequence failed", "err", err)
		return err
	}

	if err := conn.DoSpawn(); err != nil {
		s.logger.Debug("spawn sequence failed", "err", err)
		return err
	}
	s.registry.AddSession(identityData.XUID, s)
	s.logger.Info("logged in session")
	return
}

// Transfer initiates a transfer to a different server using the specified address.
// It sets a default timeout of 1 minute for the transfer operation.
func (s *Session) Transfer(addr string) (err error) {
	ctx, cancel := context.WithTimeout(s.ctx, time.Minute)
	defer cancel()
	return s.TransferContext(ctx, addr)
}

// TransferTimeout initiates a transfer to a different server using the specified address
// and a custom timeout duration for the transfer operation.
func (s *Session) TransferTimeout(addr string, duration time.Duration) (err error) {
	ctx, cancel := context.WithTimeout(s.ctx, duration)
	defer cancel()
	return s.TransferContext(ctx, addr)
}

// TransferContext initiates a transfer to a different server using the specified address. It ensures that only one transfer
// occurs at a time, returning an error if another transfer is already in progress.
// The process is performed using the provided context for cancellation.
func (s *Session) TransferContext(ctx context.Context, addr string) (err error) {
	if !s.transferring.CompareAndSwap(false, true) {
		return errors.New("already transferring")
	}

	defer s.transferring.Store(false)
	processorCtx := NewContext()
	s.Processor().ProcessPreTransfer(processorCtx, &s.serverAddr, &addr)
	if processorCtx.Cancelled() {
		return errors.New("processor failed")
	}

	s.serverMu.RLock()
	origin := s.serverAddr
	s.serverMu.RUnlock()
	s.sendMetadata(true)
	conn, err := s.dial(ctx, addr)
	defer func() {
		if err != nil {
			s.sendMetadata(false)
			s.Processor().ProcessTransferFailure(NewContext(), &s.serverAddr, &addr)
		}
	}()
	if err != nil {
		s.logger.Debug("dialer failed", "err", err)
		return err
	}

	if err := conn.ConnectContext(ctx); err != nil {
		conn.CloseWithError(fmt.Errorf("connection sequence failed: %w", err))
		s.logger.Debug("connection sequence failed", "err", err)
		return err
	}

	gameData := conn.GameData()
	s.animation.Play(s.client, gameData)
	s.sendGameData(conn.GameData())
	if err := conn.DoSpawn(); err != nil {
		conn.CloseWithError(fmt.Errorf("spawn sequence failed: %w", err))
		return err
	}
	s.animation.Clear(s.client, gameData)
	s.Processor().ProcessPostTransfer(NewContext(), &origin, &addr)
	s.logger.Debug("transferred session", "origin", origin, "target", addr)
	return nil
}

// Animation returns the animation set to be played during server transfers.
func (s *Session) Animation() animation.Animation {
	return s.animation
}

// SetAnimation sets the animation to be played during server transfers.
func (s *Session) SetAnimation(animation animation.Animation) {
	s.animation = animation
}

// Cache returns the current session cache.
func (s *Session) Cache() []byte {
	return s.cache.Load().([]byte)
}

// SetCache updates the session cache.
func (s *Session) SetCache(cache []byte) {
	ctx := NewContext()
	s.Processor().ProcessCache(ctx, &cache)
	if !ctx.Cancelled() {
		s.cache.Store(cache)
	}
}

// Processor returns the current processor.
func (s *Session) Processor() Processor {
	s.processorMu.RLock()
	defer s.processorMu.RUnlock()
	return s.processor
}

// SetProcessor sets a new processor for the session.
func (s *Session) SetProcessor(processor Processor) {
	s.processorMu.Lock()
	s.processor = processor
	s.processorMu.Unlock()
}

// Latency returns the total latency experienced by the session, combining client and server latencies.
// The client's latency is derived from half of RakNet's round-trip time (RTT).
// To calculate the total latency, we multiply this value by 2.
func (s *Session) Latency() int64 {
	return (s.client.Latency().Milliseconds() * 2) + s.latency.Load()
}

// Client returns the client connection.
func (s *Session) Client() *minecraft.Conn {
	return s.client
}

// Server returns the current server connection.
func (s *Session) Server() *server.Conn {
	s.serverMu.RLock()
	defer s.serverMu.RUnlock()
	return s.serverConn
}

// Context returns the connection's context. The context is canceled when the session is closed,
// allowing for cancellation of operations that are tied to the lifecycle of the session.
func (s *Session) Context() context.Context {
	return s.ctx
}

// Disconnect sends a packet.Disconnect to the client and closes the session.
func (s *Session) Disconnect(message string) {
	s.CloseWithError(errors.New(message))
}

// Close closes the session, including the server and client connections.
func (s *Session) Close() (err error) {
	s.CloseWithError(errors.New("closed by application"))
	return nil
}

func (s *Session) CloseWithError(err error) {
	s.once.Do(func() {
		_ = s.client.WritePacket(&packet.Disconnect{Message: err.Error()})
		_ = s.client.Close()
		s.Processor().ProcessDisconnection(NewContext(), err.Error())
		s.serverMu.RLock()
		if s.serverConn != nil {
			s.serverConn.CloseWithError(err)
		}
		s.serverMu.RUnlock()
		s.registry.RemoveSession(s.client.IdentityData().XUID)
		s.logger.Info("closed session")
	})
}

// dial dials the specified server address and returns a new server.Conn instance.
// The provided context is used to manage timeouts and cancellations during the dialing process.
func (s *Session) dial(ctx context.Context, addr string) (*server.Conn, error) {
	select {
	case <-s.ctx.Done():
		return nil, errors.New("session is closed")
	default:
	}

	s.serverMu.Lock()
	defer s.serverMu.Unlock()
	if s.serverConn != nil {
		_ = s.serverConn.Close()
	}

	conn, err := s.transport.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	c := server.NewConn(conn, s.client, s.logger.With("addr", addr), s.opts.SyncProtocol, s.Cache())
	s.serverAddr = addr
	s.serverConn = c
	return c, nil
}

// fallback attempts to transfer the session to a fallback server provided by the discovery.
func (s *Session) fallback() {
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	if !s.fallbackInProcess.CompareAndSwap(false, true) {
		return
	}

	defer s.fallbackInProcess.Store(false)
	addr, err := s.discovery.DiscoverFallback(s.client)
	if err != nil {
		s.CloseWithError(err)
		return
	}

	if err := s.Transfer(addr); err != nil {
		s.CloseWithError(fmt.Errorf("failed to transfer to fallback server: %w", err))
		return
	}
	s.logger.Info("transferred session to a fallback server", "addr", addr)
}

func (s *Session) sendMetadata(noAI bool) {
	metadata := protocol.NewEntityMetadata()
	if noAI {
		metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagNoAI)
	}
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagBreathing)
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagHasGravity)
	_ = s.client.WritePacket(&packet.SetActorData{
		EntityRuntimeID: s.client.GameData().EntityRuntimeID,
		EntityMetadata:  metadata,
	})
}

func (s *Session) sendGameData(gameData minecraft.GameData) {
	chunk := emptyChunk(gameData.Dimension)
	pos := gameData.PlayerPosition
	chunkX := int32(pos.X()) >> 4
	chunkZ := int32(pos.Z()) >> 4
	for x := chunkX - 4; x <= chunkX+4; x++ {
		for z := chunkZ - 4; z <= chunkZ+4; z++ {
			_ = s.client.WritePacket(&packet.LevelChunk{
				Dimension:     gameData.Dimension,
				Position:      protocol.ChunkPos{x, z},
				SubChunkCount: 1,
				RawPayload:    chunk,
			})
		}
	}
	s.tracker.mu.Lock()
	s.tracker.clearEffects(s)
	s.tracker.clearEntities(s)
	s.tracker.clearBossBars(s)
	s.tracker.clearPlayers(s)
	s.tracker.clearScoreboards(s)
	s.tracker.mu.Unlock()
	_ = s.client.WritePacket(&packet.MovePlayer{
		EntityRuntimeID: gameData.EntityRuntimeID,
		Position:        gameData.PlayerPosition,
		Pitch:           gameData.Pitch,
		Yaw:             gameData.Yaw,
		Mode:            packet.MoveModeReset,
	})
	_ = s.client.WritePacket(&packet.LevelEvent{EventType: packet.LevelEventStopRaining, EventData: 10_000})
	_ = s.client.WritePacket(&packet.LevelEvent{EventType: packet.LevelEventStopThunderstorm})
	_ = s.client.WritePacket(&packet.SetDifficulty{Difficulty: uint32(gameData.Difficulty)})
	_ = s.client.WritePacket(&packet.SetPlayerGameType{GameType: gameData.PlayerGameMode})
	_ = s.client.WritePacket(&packet.GameRulesChanged{GameRules: gameData.GameRules})
}
