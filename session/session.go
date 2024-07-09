package session

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

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

	loggedIn     atomic.Bool
	transferring atomic.Bool

	ch     chan struct{}
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

		ch: make(chan struct{}),
	}
	s.serverMu.Lock()
	return s
}

// Login initiates the login process, including server discovery, connection, and player spawning.
func (s *Session) Login() (err error) {
	defer s.serverMu.Unlock()

	go handleIncoming(s)
	go handleOutgoing(s)
	go handleLatency(s, s.opts.LatencyInterval)

	serverAddr, err := s.discovery.Discover(s.clientConn)
	if err != nil {
		return fmt.Errorf("discovery failed: %v", err)
	}

	serverConn, err := s.dial(serverAddr)
	if err != nil {
		return fmt.Errorf("dialer failed: %v", err)
	}

	s.serverAddr = serverAddr
	s.serverConn = serverConn
	if err := serverConn.Connect(); err != nil {
		return fmt.Errorf("connection sequence failed: %v", err)
	}

	if err := serverConn.Spawn(); err != nil {
		return fmt.Errorf("spawn sequence failed: %v", err)
	}

	gameData := serverConn.GameData()
	s.processor.ProcessStartGame(NewContext(), &gameData)
	if err := s.clientConn.StartGame(gameData); err != nil {
		return fmt.Errorf("startgame sequence failed: %v", err)
	}

	identityData := s.clientConn.IdentityData()
	s.sendMetadata(true)
	s.loggedIn.Store(true)
	s.registry.AddSession(identityData.XUID, s)
	s.logger.Info("logged in session", "username", identityData.DisplayName)
	return
}

// Transfer moves the session to a different server. It ensures that only one transfer occurs at a time,
// returning an error if another transfer is in progress.
func (s *Session) Transfer(addr string) (err error) {
	if !s.transferring.CompareAndSwap(false, true) {
		return errors.New("already transferring")
	}

	defer s.transferring.Store(false)

	ctx := NewContext()
	s.processor.ProcessPreTransfer(ctx, &s.serverAddr, &addr)
	if ctx.Cancelled() {
		return errors.New("processor failed")
	}

	if s.serverAddr == addr {
		return errors.New("already connected to this server")
	}

	s.serverMu.Lock()
	defer func() {
		if err != nil {
			s.serverMu.Unlock()
		}
	}()

	conn, err := s.dial(addr)
	if err != nil {
		return fmt.Errorf("dialer failed: %v", err)
	}

	s.sendMetadata(true)
	if err := conn.Connect(); err != nil {
		_ = conn.Close()
		s.sendMetadata(false)
		return fmt.Errorf("connection sequence failed: %v", err)
	}

	if err := conn.Spawn(); err != nil {
		_ = conn.Close()
		s.sendMetadata(false)
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
func (s *Session) Latency() int64 {
	return s.clientConn.Latency().Milliseconds() + s.serverLatency
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
		close(s.ch)
		s.closed.Store(true)
		s.processor.ProcessDisconnection(NewContext())

		_ = s.clientConn.Close()
		if s.serverConn != nil {
			_ = s.serverConn.Close()
		}

		identity := s.clientConn.IdentityData()
		s.registry.RemoveSession(identity.XUID)
		if s.loggedIn.Load() {
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
