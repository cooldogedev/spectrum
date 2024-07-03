package session

import (
	"errors"
	"net"
	"strings"
	"time"

	packet2 "github.com/cooldogedev/spectrum/server/packet"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	errClosedNetworkConn = "closed network connection"
	errClosedStream      = "closed stream"
)

func handleIncoming(s *Session) {
	defer s.Close()
	for {
		select {
		case <-s.ch:
			return
		default:
			if s.transferring.Load() {
				continue
			}

			server := s.Server()
			pk, err := server.ReadPacket()
			if err != nil {
				if server != s.Server() {
					continue
				}

				if !s.closed.Load() {
					if isErrorLoggable(err) {
						s.logger.Error("failed to read packet from server", "username", s.clientConn.IdentityData().DisplayName, "err", err)
					}

					if err := s.fallback(); err != nil {
						s.logger.Error("fallback failed", "username", s.clientConn.IdentityData().DisplayName, "err", err)
					} else {
						continue
					}
				}
				return
			}

			switch pk := pk.(type) {
			case *packet2.Latency:
				s.serverLatency = pk.Latency
			case *packet2.Transfer:
				if err := s.Transfer(pk.Addr); err != nil {
					s.logger.Error("failed to transfer", "err", err)
				}
			case packet.Packet:
				ctx := NewContext()
				s.processor.ProcessServer(ctx, pk)
				if ctx.Cancelled() {
					continue
				}

				s.tracker.handlePacket(pk)
				if err := s.clientConn.WritePacket(pk); err != nil && isErrorLoggable(err) {
					s.logger.Error("failed to write packet to client", "err", err)
				} else {
					continue
				}
				return
			case []byte:
				if _, err := s.clientConn.Write(pk); err != nil && isErrorLoggable(err) {
					s.logger.Error("failed to write raw packet to client", "err", err)
				} else {
					continue
				}
				return
			}
		}
	}
}

func handleOutgoing(s *Session) {
	defer s.Close()
	for {
		select {
		case <-s.ch:
			return
		default:
			pk, err := s.clientConn.ReadPacket()
			if err != nil {
				if isErrorLoggable(err) {
					s.logger.Error("failed to read packet from client", "err", err)
				}
				return
			}

			ctx := NewContext()
			if s.transferring.Load() {
				ctx.Cancel()
			}

			s.processor.ProcessClient(ctx, pk)
			if ctx.Cancelled() {
				continue
			}

			if err := s.Server().WritePacket(pk); err != nil && isErrorLoggable(err) {
				s.logger.Error("failed to write packet to server", "err", err)
			}
		}
	}
}

func handleLatency(s *Session, interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	defer func() {
		_ = s.Close()
		ticker.Stop()
	}()
	for {
		select {
		case <-s.ch:
			return
		case <-ticker.C:
			if s.transferring.Load() {
				continue
			}

			err := s.Server().WritePacket(&packet2.Latency{
				Latency:   s.clientConn.Latency().Milliseconds(),
				Timestamp: time.Now().UnixMilli(),
			})
			if err != nil && !s.closed.Load() {
				s.logger.Error("failed to send latency packet", "err", err)
			}
		}
	}
}

func isErrorLoggable(err error) bool {
	return !errors.Is(err, net.ErrClosed) && !strings.Contains(err.Error(), errClosedStream) && !strings.Contains(err.Error(), errClosedNetworkConn)
}
