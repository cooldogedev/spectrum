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
	clientConn *minecraft.Conn

	serverAddr    string
	serverConn    *server.Conn
	serverLatency int64
	serverMu      sync.RWMutex

	logger   *slog.Logger
	registry *Registry

	discovery server.Discovery
	opts      util.Opts
	transport transport.Transport

	animation animation.Animation
	processor Processor
	tracker   *tracker

	loggedIn     bool
	transferring atomic.Bool

	ctx        context.Context
	cancelFunc context.CancelFunc

	closed atomic.Bool
	once   sync.Once
}

// NewSession creates a new Session instance using the provided minecraft.Conn.
func NewSession(clientConn *minecraft.Conn, logger *slog.Logger, registry *Registry, discovery server.Discovery, opts util.Opts, transport transport.Transport) *Session {
	s := &Session{
		clientConn: clientConn,

		logger:   logger,
		registry: registry,

		discovery: discovery,
		opts:      opts,
		transport: transport,

		animation: &animation.Dimension{},
		processor: NopProcessor{},
		tracker:   newTracker(),
	}
	s.ctx, s.cancelFunc = context.WithCancel(context.Background())
	s.serverMu.Lock()
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
	defer s.serverMu.Unlock()

	go handleServer(s)
	go handleClient(s)
	go handleLatency(s, s.opts.LatencyInterval)

	serverAddr, err := s.discovery.Discover(s.clientConn)
	if err != nil {
		return fmt.Errorf("discovery failed: %v", err)
	}

	serverConn, err := s.dial(serverAddr)
	if err != nil {
		s.logger.Debug("dialer failed", "err", err)
		return err
	}

	s.serverAddr = serverAddr
	s.serverConn = serverConn
	if err := serverConn.ConnectContext(ctx); err != nil {
		s.logger.Debug("connection sequence failed", "err", err)
		return err
	}

	if err := serverConn.SpawnContext(ctx); err != nil {
		s.logger.Debug("spawn sequence failed", "err", err)
		return err
	}

	gameData := serverConn.GameData()
	s.processor.ProcessStartGame(NewContext(), &gameData)
	if err := s.clientConn.StartGame(gameData); err != nil {
		s.logger.Debug("startgame sequence failed", "err", err)
		return err
	}

	identityData := s.clientConn.IdentityData()
	s.loggedIn = true
	s.registry.AddSession(identityData.XUID, s)
	s.logger.Info("logged in session", "username", identityData.DisplayName)
	return
}

// Transfer initiates a transfer to a different server using the specified address.
// It sets a default timeout of 1 minute for the transfer operation.
func (s *Session) Transfer(addr string) (err error) {
	ctx, cancel := context.WithTimeout(s.ctx, time.Minute)
	defer cancel()
	return s.TransferContext(addr, ctx)
}

// TransferTimeout initiates a transfer to a different server using the specified address
// and a custom timeout duration for the transfer operation.
func (s *Session) TransferTimeout(addr string, duration time.Duration) (err error) {
	ctx, cancel := context.WithTimeout(s.ctx, duration)
	defer cancel()
	return s.TransferContext(addr, ctx)
}

// TransferContext initiates a transfer to a different server using the specified address. It ensures that only one transfer
// occurs at a time, returning an error if another transfer is already in progress.
// The process is performed using the provided context for cancellation.
func (s *Session) TransferContext(addr string, ctx context.Context) (err error) {
	if !s.transferring.CompareAndSwap(false, true) {
		return errors.New("already transferring")
	}

	defer s.transferring.Store(false)

	processorCtx := NewContext()
	s.processor.ProcessPreTransfer(processorCtx, &s.serverAddr, &addr)
	if processorCtx.Cancelled() {
		return errors.New("processor failed")
	}

	if s.serverAddr == addr {
		return errors.New("already connected to this server")
	}

	s.serverMu.Lock()
	defer func() {
		if err != nil {
			s.sendMetadata(false)
			s.serverMu.Unlock()
		}
	}()

	conn, err := s.dial(addr)
	if err != nil {
		return fmt.Errorf("dialer failed: %v", err)
	}

	s.sendMetadata(true)
	if err := conn.ConnectContext(ctx); err != nil {
		_ = conn.Close()
		return fmt.Errorf("connection sequence failed: %v", err)
	}

	if err := conn.SpawnContext(ctx); err != nil {
		_ = conn.Close()
		return fmt.Errorf("spawn sequence failed: %v", err)
	}

	_ = s.serverConn.Close()
	serverGameData := conn.GameData()
	s.animation.Play(s.clientConn, serverGameData)

	chunk := emptyChunk(serverGameData.Dimension)
	pos := serverGameData.PlayerPosition
	chunkX := int32(pos.X()) >> 4
	chunkZ := int32(pos.Z()) >> 4
	for x := chunkX - 4; x <= chunkX+4; x++ {
		for z := chunkZ - 4; z <= chunkZ+4; z++ {
			_ = s.clientConn.WritePacket(&packet.LevelChunk{
				Dimension:     packet.DimensionNether,
				Position:      protocol.ChunkPos{x, z},
				SubChunkCount: 1,
				RawPayload:    chunk,
			})
		}
	}

	s.tracker.clearEffects(s)
	s.tracker.clearEntities(s)
	s.tracker.clearBossBars(s)
	s.tracker.clearPlayers(s)
	s.tracker.clearScoreboards(s)

	_ = s.clientConn.WritePacket(&packet.MovePlayer{
		EntityRuntimeID: serverGameData.EntityRuntimeID,
		Position:        serverGameData.PlayerPosition,
		Pitch:           serverGameData.Pitch,
		Yaw:             serverGameData.Yaw,
		Mode:            packet.MoveModeReset,
	})
	_ = s.clientConn.WritePacket(&packet.LevelEvent{EventType: packet.LevelEventStopRaining, EventData: 10_000})
	_ = s.clientConn.WritePacket(&packet.LevelEvent{EventType: packet.LevelEventStopThunderstorm})
	_ = s.clientConn.WritePacket(&packet.SetDifficulty{Difficulty: uint32(serverGameData.Difficulty)})
	_ = s.clientConn.WritePacket(&packet.SetPlayerGameType{GameType: serverGameData.PlayerGameMode})
	_ = s.clientConn.WritePacket(&packet.GameRulesChanged{GameRules: serverGameData.GameRules})

	s.animation.Clear(s.clientConn, serverGameData)
	s.serverAddr = addr
	s.serverConn = conn
	s.serverMu.Unlock()

	s.processor.ProcessPostTransfer(NewContext(), &s.serverAddr, &addr)
	s.logger.Debug("transferred session", "username", s.clientConn.IdentityData().DisplayName, "addr", addr)
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

// Opts returns the current session options.
func (s *Session) Opts() util.Opts {
	return s.opts
}

// SetOpts updates the session options.
func (s *Session) SetOpts(opts util.Opts) {
	s.opts = opts
}

// Processor returns the current processor.
func (s *Session) Processor() Processor {
	return s.processor
}

// SetProcessor sets a new processor for the session.
func (s *Session) SetProcessor(processor Processor) {
	s.processor = processor
}

// Latency returns the total latency experienced by the session, combining client and server latencies.
// Note: The client's latency is derived from half of RakNet's round-trip time (RTT).
// To calculate the total latency, we multiply this value by 2.
func (s *Session) Latency() int64 {
	return (s.clientConn.Latency().Milliseconds() * 2) + s.serverLatency
}

// Client returns the client connection.
func (s *Session) Client() *minecraft.Conn {
	return s.clientConn
}

// Server returns the current server connection.
func (s *Session) Server() *server.Conn {
	s.serverMu.RLock()
	defer s.serverMu.RUnlock()
	return s.serverConn
}

// Disconnect sends a packet.Disconnect to the client and closes the session.
func (s *Session) Disconnect(message string) {
	_ = s.clientConn.WritePacket(&packet.Disconnect{Message: message})
	_ = s.Close()
}

// Close closes the session, including the server and client connections.
func (s *Session) Close() (err error) {
	s.once.Do(func() {
		s.closed.Store(true)
		if s.cancelFunc != nil {
			s.cancelFunc()
		}

		s.processor.ProcessDisconnection(NewContext())
		_ = s.clientConn.Close()
		if s.serverConn != nil {
			_ = s.serverConn.Close()
		}

		identity := s.clientConn.IdentityData()
		s.registry.RemoveSession(identity.XUID)
		if s.loggedIn {
			s.logger.Info("closed session", "username", identity.DisplayName)
		} else {
			s.logger.Debug("closed unlogged session", "username", identity.DisplayName)
		}
	})
	return
}

// dial establishes a connection to the specified server address and returns a new server.Conn instance.
func (s *Session) dial(addr string) (*server.Conn, error) {
	conn, err := s.transport.Dial(addr)
	if err != nil {
		return nil, err
	}
	return server.NewConn(conn, s.clientConn.RemoteAddr(), s.opts.Token, s.clientConn.ClientData(), s.clientConn.IdentityData(), packet.NewServerPool()), nil
}

// fallback attempts to transfer the session to a fallback server provided by the discovery.
func (s *Session) fallback() (err error) {
	addr, err := s.discovery.DiscoverFallback(s.clientConn)
	if err != nil {
		return err
	}

	if err := s.Transfer(addr); err != nil {
		return err
	}
	s.logger.Debug("transferred session to a fallback server", "username", s.clientConn.IdentityData().DisplayName, "addr", addr)
	return
}

// sendMetadata toggles the player's immobility during transfers to prevent position mismatches
// between the client and the server.
func (s *Session) sendMetadata(noAI bool) {
	metadata := protocol.NewEntityMetadata()
	if noAI {
		metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagNoAI)
	}
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagBreathing)
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagHasGravity)
	_ = s.clientConn.WritePacket(&packet.SetActorData{
		EntityRuntimeID: s.clientConn.GameData().EntityRuntimeID,
		EntityMetadata:  metadata,
	})
}
