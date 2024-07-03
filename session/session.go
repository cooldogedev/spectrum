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
	tracker   *Tracker

	loggedIn     atomic.Bool
	transferring atomic.Bool

	ch     chan struct{}
	closed atomic.Bool
	once   sync.Once
}

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
		tracker:   NewTracker(),

		ch: make(chan struct{}),
	}
	s.serverMu.Lock()
	return s
}

func (s *Session) Login() (err error) {
	defer s.serverMu.Unlock()

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
	if err := serverConn.Connect(s.clientConn, s.opts.Token); err != nil {
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

	s.sendMetadata(true)

	go handleIncoming(s)
	go handleOutgoing(s)
	go handleLatency(s, s.opts.LatencyInterval)

	identityData := s.clientConn.IdentityData()
	s.loggedIn.Store(true)
	s.registry.AddSession(identityData.XUID, s)
	s.logger.Info("logged in session", "username", identityData.DisplayName)
	return
}

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
	if err := conn.Connect(s.clientConn, s.opts.Token); err != nil {
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

func (s *Session) Animation() animation.Animation {
	return s.animation
}

func (s *Session) SetAnimation(animation animation.Animation) {
	s.animation = animation
}

func (s *Session) Opts() util.Opts {
	return s.opts
}

func (s *Session) SetOpts(opts util.Opts) {
	s.opts = opts
}

func (s *Session) Processor() Processor {
	return s.processor
}

func (s *Session) SetProcessor(processor Processor) {
	s.processor = processor
}

func (s *Session) Latency() int64 {
	return s.clientConn.Latency().Milliseconds() + s.serverLatency
}

func (s *Session) Client() *minecraft.Conn {
	return s.clientConn
}

func (s *Session) Server() *server.Conn {
	s.serverMu.RLock()
	defer s.serverMu.RUnlock()
	return s.serverConn
}

func (s *Session) Disconnect(message string) {
	_ = s.clientConn.WritePacket(&packet.Disconnect{Message: message})
	_ = s.Close()
}

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

func (s *Session) dial(addr string) (*server.Conn, error) {
	conn, err := s.transport.Dial(addr)
	if err != nil {
		return nil, err
	}
	return server.NewConn(conn, packet.NewServerPool()), nil
}

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
