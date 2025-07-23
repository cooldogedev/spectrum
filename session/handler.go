package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	spectrumpacket "github.com/cooldogedev/spectrum/server/packet"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// handleServer continuously reads packets from the server and forwards them to the client.
func handleServer(s *Session) {
loop:
	for {
		select {
		case <-s.ctx.Done():
			s.CloseWithError(context.Cause(s.ctx))
			break loop
		default:
		}

		server := s.Server()
		pk, err := server.ReadPacket()
		if err != nil {
			if server != s.Server() {
				continue loop
			}

			server.CloseWithError(fmt.Errorf("failed to read packet from server: %w", err))
			if err := s.fallback(); err != nil {
				s.CloseWithError(fmt.Errorf("fallback failed: %w", err))
				break loop
			}
			continue loop
		}

		switch pk := pk.(type) {
		case *spectrumpacket.Flush:
			ctx := NewContext()
			s.Processor().ProcessFlush(ctx)
			if ctx.Cancelled() {
				continue loop
			}

			if err := s.client.Flush(); err != nil {
				s.CloseWithError(fmt.Errorf("failed to flush client's buffer: %w", err))
				logError(s, "failed to flush client's buffer", err)
				break loop
			}
		case *spectrumpacket.Latency:
			s.latency.Store(pk.Latency)
		case *spectrumpacket.Transfer:
			if err := s.Transfer(pk.Addr); err != nil {
				logError(s, "failed to transfer", err)
			}
		case *spectrumpacket.UpdateCache:
			s.SetCache(pk.Cache)
		case packet.Packet:
			if err := handleServerPacket(s, pk); err != nil {
				s.CloseWithError(fmt.Errorf("failed to write packet to client: %w", err))
				logError(s, "failed to write packet to client", err)
				break loop
			}
		case []byte:
			ctx := NewContext()
			s.Processor().ProcessServerEncoded(ctx, &pk)
			if ctx.Cancelled() {
				continue loop
			}

			if _, err := s.client.Write(pk); err != nil {
				s.CloseWithError(fmt.Errorf("failed to write packet to client: %w", err))
				logError(s, "failed to write packet to client", err)
				break loop
			}
		}
	}
}

// handleClient continuously reads packets from the client and forwards them to the server.
func handleClient(s *Session) {
	header := &packet.Header{}
	pool := s.client.Proto().Packets(true)
	var shieldID int32
	for _, item := range s.client.GameData().Items {
		if item.Name == "minecraft:shield" {
			shieldID = int32(item.RuntimeID)
			break
		}
	}

loop:
	for {
		select {
		case <-s.ctx.Done():
			s.CloseWithError(context.Cause(s.ctx))
			break loop
		default:
		}

		payload, err := s.client.ReadBytes()
		if err != nil {
			s.CloseWithError(fmt.Errorf("failed to read packet from client: %w", err))
			logError(s, "failed to read packet from client", err)
			break loop
		}

		if err := handleClientPacket(s, header, pool, shieldID, payload); err != nil {
			s.Server().CloseWithError(fmt.Errorf("failed to write packet to server: %w", err))
		}
	}
}

// handleLatency periodically sends the client's current ping and timestamp to the server for latency reporting.
// The client's latency is derived from half of RakNet's round-trip time (RTT).
// To calculate the total latency, we multiply this value by 2.
func handleLatency(s *Session, interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	defer ticker.Stop()
loop:
	for {
		select {
		case <-s.ctx.Done():
			s.CloseWithError(context.Cause(s.ctx))
			break loop
		case <-ticker.C:
			if err := s.Server().WritePacket(&spectrumpacket.Latency{Latency: s.client.Latency().Milliseconds() * 2, Timestamp: time.Now().UnixMilli()}); err != nil {
				logError(s, "failed to write latency packet", err)
			}
		}
	}
}

// handleServerPacket processes and forwards the provided packet from the server to the client.
func handleServerPacket(s *Session, pk packet.Packet) (err error) {
	ctx := NewContext()
	s.Processor().ProcessServer(ctx, &pk)
	if ctx.Cancelled() {
		return
	}

	if s.opts.SyncProtocol {
		for _, latest := range s.client.Proto().ConvertToLatest(pk, s.client) {
			s.tracker.handlePacket(latest)
		}
	} else {
		s.tracker.handlePacket(pk)
	}
	return s.client.WritePacket(pk)
}

// handleClientPacket processes and forwards the provided packet from the client to the server.
func handleClientPacket(s *Session, header *packet.Header, pool packet.Pool, shieldID int32, payload []byte) (err error) {
	ctx := NewContext()
	buf := bytes.NewBuffer(payload)
	if err := header.Read(buf); err != nil {
		return errors.New("failed to decode header")
	}

	if !slices.Contains(s.opts.ClientDecode, header.PacketID) {
		s.Processor().ProcessClientEncoded(ctx, &payload)
		if !ctx.Cancelled() {
			return s.Server().Write(payload)
		}
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet %v: %v", header.PacketID, r)
		}
	}()

	factory, ok := pool[header.PacketID]
	if !ok {
		return fmt.Errorf("unknown packet %d", header.PacketID)
	}

	pk := factory()
	pk.Marshal(s.client.Proto().NewReader(buf, shieldID, true))
	if s.opts.SyncProtocol {
		s.Processor().ProcessClient(ctx, &pk)
		if ctx.Cancelled() {
			return
		}
		return s.Server().WritePacket(pk)
	}

	for _, latest := range s.client.Proto().ConvertToLatest(pk, s.client) {
		s.Processor().ProcessClient(ctx, &latest)
		if ctx.Cancelled() {
			break
		}

		if err := s.Server().WritePacket(latest); err != nil {
			return err
		}
	}
	return
}

func logError(s *Session, msg string, err error) {
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	if !errors.Is(err, context.Canceled) {
		s.logger.Error(msg, "err", err)
	}
}
