package parser

import "github.com/dotabuff/manta"

// Map geometry: entity positions are stored as a 128-unit cell index plus an
// offset inside the cell; the world origin sits at cell 128 (16384 units).
const (
	cellSize  = 128.0
	mapOffset = 16384.0

	// Playable world bounds (approximate, Dota 7.3x+ map).
	WorldMin = -8288.0
	WorldMax = 8288.0
)

// NullHandle is the "no entity" handle value.
const NullHandle = 0xFFFFFF

// num reads any integer/float field as float64.
func num(e *manta.Entity, field string) (float64, bool) {
	switch v := e.Get(field).(type) {
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case uint8:
		return float64(v), true
	case int8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case int16:
		return float64(v), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// Int reads any numeric field as int (0 if missing).
func Int(e *manta.Entity, field string) int {
	v, _ := num(e, field)
	return int(v)
}

// Float reads any numeric field as float64 (0 if missing).
func Float(e *manta.Entity, field string) float64 {
	v, _ := num(e, field)
	return v
}

// Handle reads an entity handle field (NullHandle if missing).
func Handle(e *manta.Entity, field string) uint64 {
	v, ok := num(e, field)
	if !ok {
		return NullHandle
	}
	return uint64(v)
}

// WorldPos returns the entity's world coordinates from CBodyComponent.
func WorldPos(e *manta.Entity) (x, y float64, ok bool) {
	cx, ok1 := num(e, "CBodyComponent.m_cellX")
	cy, ok2 := num(e, "CBodyComponent.m_cellY")
	vx, _ := num(e, "CBodyComponent.m_vecX")
	vy, _ := num(e, "CBodyComponent.m_vecY")
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return cx*cellSize + vx - mapOffset, cy*cellSize + vy - mapOffset, true
}
