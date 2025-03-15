package session

import (
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/scylladb/go-set/b16set"
	"github.com/scylladb/go-set/i32set"
	"github.com/scylladb/go-set/i64set"
	"github.com/scylladb/go-set/strset"
)

type tracker struct {
	bossBars    *i64set.Set
	effects     *i32set.Set
	entities    *i64set.Set
	players     *b16set.Set
	scoreboards *strset.Set
	mu          sync.Mutex
}

func newTracker() *tracker {
	return &tracker{
		bossBars:    i64set.New(),
		effects:     i32set.New(),
		entities:    i64set.New(),
		players:     b16set.New(),
		scoreboards: strset.New(),
	}
}

func (t *tracker) handlePacket(pk packet.Packet) {
	t.mu.Lock()
	defer t.mu.Unlock()
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
		} else if pk.Operation == packet.MobEffectRemove {
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

func (t *tracker) clearAll(s *Session) {
	t.mu.Lock()
	t.clearEffects(s)
	t.clearEntities(s)
	t.clearBossBars(s)
	t.clearPlayers(s)
	t.clearScoreboards(s)
	t.mu.Unlock()
}

func (t *tracker) clearBossBars(s *Session) {
	t.bossBars.Each(func(i int64) bool {
		_ = s.client.WritePacket(&packet.BossEvent{
			BossEntityUniqueID: i,
			EventType:          packet.BossEventHide,
		})
		return true
	})
	t.bossBars.Clear()
}

func (t *tracker) clearEffects(s *Session) {
	t.effects.Each(func(i int32) bool {
		_ = s.client.WritePacket(&packet.MobEffect{
			EntityRuntimeID: s.client.GameData().EntityRuntimeID,
			EffectType:      i,
			Operation:       packet.MobEffectRemove,
		})
		return true
	})
	t.effects.Clear()
}

func (t *tracker) clearEntities(s *Session) {
	t.entities.Each(func(i int64) bool {
		_ = s.client.WritePacket(&packet.RemoveActor{
			EntityUniqueID: i,
		})
		return true
	})
	t.entities.Clear()
}

func (t *tracker) clearPlayers(s *Session) {
	entries := make([]protocol.PlayerListEntry, 0)
	t.players.Each(func(i [16]byte) bool {
		entries = append(entries, protocol.PlayerListEntry{
			UUID: i,
		})
		return true
	})
	t.players.Clear()

	_ = s.client.WritePacket(&packet.PlayerList{
		ActionType: packet.PlayerListActionRemove,
		Entries:    entries,
	})
}

func (t *tracker) clearScoreboards(s *Session) {
	t.scoreboards.Each(func(i string) bool {
		_ = s.client.WritePacket(&packet.RemoveObjective{
			ObjectiveName: i,
		})
		return true
	})
	t.scoreboards.Clear()
}
