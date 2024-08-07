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

// handleServer continuously reads packets from the server and forwards them to the client.
func handleServer(s *Session) {
	defer s.Close()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			server := s.Server()
			if !s.loggedIn {
				continue
			}

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
				if err := s.clientConn.WritePacket(pk); err != nil {
					if isErrorLoggable(err) {
						s.logger.Error("failed to write packet to client", "err", err)
					}
					return
				}
			case []byte:
				if _, err := s.clientConn.Write(pk); err != nil {
					if isErrorLoggable(err) {
						s.logger.Error("failed to write raw packet to client", "err", err)
					}
					return
				}
			}
		}
	}
}

// handleClient continuously reads packets from the client and forwards them to the server.
func handleClient(s *Session) {
	defer s.Close()
	var deferredPackets []packet.Packet
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			pk, err := s.clientConn.ReadPacket()
			if err != nil {
				if isErrorLoggable(err) {
					s.logger.Error("failed to read packet from client", "err", err)
				}
				return
			}

			if !s.loggedIn {
				deferredPackets = append(deferredPackets, pk)
				continue
			}

			if len(deferredPackets) > 0 {
				for _, deferredPacket := range deferredPackets {
					handleClientPacket(s, deferredPacket)
				}
				deferredPackets = nil
			}
			handleClientPacket(s, pk)
		}
	}
}

// handleLatency periodically sends the client's current ping and timestamp to the server for latency reporting.
// Note: The client's latency is derived from half of RakNet's round-trip time (RTT).
// To calculate the total latency, we multiply this value by 2.
func handleLatency(s *Session, interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	defer func() {
		_ = s.Close()
		ticker.Stop()
	}()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if !s.loggedIn {
				continue
			}

			err := s.Server().WritePacket(&packet2.Latency{
				Latency:   s.clientConn.Latency().Milliseconds() * 2,
				Timestamp: time.Now().UnixMilli(),
			})
			if err != nil && !s.closed.Load() {
				s.logger.Error("failed to send latency packet", "err", err)
			}
		}
	}
}

// handleClientPacket processes and forwards the provided packet from the client to the server.
func handleClientPacket(s *Session, pk packet.Packet) {
	ctx := NewContext()
	if s.transferring.Load() {
		ctx.Cancel()
	}

	s.processor.ProcessClient(ctx, pk)
	if ctx.Cancelled() {
		return
	}

	if err := s.Server().WritePacket(pk); err != nil && isErrorLoggable(err) {
		s.logger.Error("failed to write packet to server", "err", err)
	}
}

func isErrorLoggable(err error) bool {
	return !errors.Is(err, net.ErrClosed) && !strings.Contains(err.Error(), errClosedStream) && !strings.Contains(err.Error(), errClosedNetworkConn)
}
