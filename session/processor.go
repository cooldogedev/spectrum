package session

import (
	"errors"
	"github.com/spectrum-proxy/spectrum/event"
	"github.com/spectrum-proxy/spectrum/server/packet"
	"net"
	"strings"
	"time"
)

func handleIncoming(s *Session) {
	defer s.Close()

	for {
		if s.transferring.Load() {
			continue
		}

		server := s.Server()
		pk, err := server.ReadPacket()
		if err != nil {
			if server != s.Server() {
				continue
			}

			if !errors.Is(err, net.ErrClosed) {
				s.logger.Errorf("Failed to read packet from server: %v", err)
			}
			return
		}

		switch pk := pk.(type) {
		case *packet.Latency:
			s.latency = pk.Latency
		case *packet.Transfer:
			if err := s.Transfer(pk.Addr); err != nil {
				s.logger.Errorf("Failed to transfer: %v", err)
			}
		default:
			ctx := event.New()
			s.handler.HandleIncoming(ctx, pk)

			if ctx.Cancelled() {
				return
			}

			if err := s.clientConn.WritePacket(pk); err != nil {
				s.logger.Errorf("Failed to write packet to client: %v", err)
				return
			}
		}
	}
}

func handleOutgoing(s *Session) {
	defer s.Close()

	for {
		if s.transferring.Load() {
			continue
		}

		pk, err := s.clientConn.ReadPacket()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				s.logger.Errorf("Failed to read packet from client: %v", err)
			}
			return
		}

		ctx := event.New()
		s.handler.HandleOutgoing(ctx, pk)

		if ctx.Cancelled() {
			return
		}

		if err := s.Server().WritePacket(pk); err != nil {
			s.logger.Errorf("Failed to write packet to server: %v", err)
			return
		}
	}
}

func handleLatency(s *Session, interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	for range ticker.C {
		if s.transferring.Load() {
			continue
		}

		err := s.Server().WritePacket(&packet.Latency{
			Latency:   s.clientConn.Latency().Milliseconds(),
			Timestamp: time.Now().UnixMilli(),
		})
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				s.logger.Errorf("Failed to send latency packet: %v", err)
			}
			return
		}
	}
}
