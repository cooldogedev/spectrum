package session

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/scylladb/go-set/b16set"
	"github.com/scylladb/go-set/i32set"
	"github.com/scylladb/go-set/i64set"
	"github.com/scylladb/go-set/strset"
)

type Tracker struct {
	bossBars    *i64set.Set
	effects     *i32set.Set
	entities    *i64set.Set
	players     *b16set.Set
	scoreboards *strset.Set
}

func NewTracker() *Tracker {
	return &Tracker{
		bossBars:    i64set.New(),
		effects:     i32set.New(),
		entities:    i64set.New(),
		players:     b16set.New(),
		scoreboards: strset.New(),
	}
}

func (t *Tracker) handlePacket(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.AddActor:
		t.entities.Add(pk.EntityUniqueID)
	case *packet.AddItemActor:
		t.entities.Add(pk.EntityUniqueID)
	case *packet.AddPainting:
		t.entities.Add(pk.EntityUniqueID)
	case *packet.AddPlayer:
		t.entities.Add(pk.AbilityData.EntityUniqueID)
	case *packet.BossEvent:
		t.bossBars.Add(pk.BossEntityUniqueID)
	case *packet.MobEffect:
		if pk.Operation == packet.MobEffectAdd {
			t.effects.Add(pk.EffectType)
		} else {
			t.effects.Remove(pk.EffectType)
		}
	case *packet.PlayerList:
		for _, entry := range pk.Entries {
			if pk.ActionType == packet.PlayerListActionAdd {
				t.players.Add(entry.UUID)
			} else {
				t.players.Remove(entry.UUID)
			}
		}
	case *packet.RemoveActor:
		t.entities.Remove(pk.EntityUniqueID)
	case *packet.RemoveObjective:
		t.scoreboards.Remove(pk.ObjectiveName)
	case *packet.SetDisplayObjective:
		t.scoreboards.Add(pk.ObjectiveName)
	}
}

func (t *Tracker) clearBossBars(s *Session) {
	t.bossBars.Each(func(i int64) bool {
		_ = s.clientConn.WritePacket(&packet.BossEvent{
			BossEntityUniqueID: i,
		})
		return true
	})
	t.bossBars.Clear()
}

func (t *Tracker) clearEffects(s *Session) {
	t.effects.Each(func(i int32) bool {
		_ = s.clientConn.WritePacket(&packet.MobEffect{
			EntityRuntimeID: s.clientConn.GameData().EntityRuntimeID,
			EffectType:      i,
			Operation:       packet.MobEffectRemove,
		})
		return true
	})
	t.effects.Clear()
}

func (t *Tracker) clearEntities(s *Session) {
	t.entities.Each(func(i int64) bool {
		_ = s.clientConn.WritePacket(&packet.RemoveActor{
			EntityUniqueID: i,
		})
		return true
	})
	t.entities.Clear()
}

func (t *Tracker) clearPlayers(s *Session) {
	entries := make([]protocol.PlayerListEntry, 0)
	t.players.Each(func(i [16]byte) bool {
		entries = append(entries, protocol.PlayerListEntry{
			UUID: i,
		})
		return true
	})
	t.players.Clear()

	_ = s.clientConn.WritePacket(&packet.PlayerList{
		ActionType: packet.PlayerListActionRemove,
		Entries:    entries,
	})
}

func (t *Tracker) clearScoreboards(s *Session) {
	t.scoreboards.Each(func(i string) bool {
		_ = s.clientConn.WritePacket(&packet.RemoveObjective{
			ObjectiveName: i,
		})
		return true
	})

	t.scoreboards.Clear()
}
