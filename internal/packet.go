package internal

import "github.com/brentp/intintmap"

var ClientPacketMap *intintmap.Map

func ClientPacketExists(id uint32) bool {
	if ClientPacketMap == nil {
		return false
	}
	_, ok := ClientPacketMap.Get(int64(id))
	return ok
}
