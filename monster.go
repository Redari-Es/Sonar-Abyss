package main

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type MonsterState int

const (
	StateWander      MonsterState = iota
	StateInvestigate              // 听到声音，前往调查
	StateChase                    // 近距离感知玩家，追击
)

type Monster struct {
	x, y   float32
	field  [mapW][mapH]int // 寻路场
	repath int

	state      MonsterState
	hasTarget  bool
	tgtX, tgtY int // 最后听到声音的位置

	wX, wY float32 // 游荡目标
	wTimer int

	pingIn int //距离下次发射自身脉冲tick数
}

func newMonster(x, y float32) *Monster {
	return &Monster{
		x: x, y: y, wX: x, wY: y,
		pingIn: 180 + rand.Intn(240),
	}
}

func (m *Monster) tile() (int, int) {
	return int((m.x + 6) / tileSize), int((m.y + 6) / tileSize)
}

// hear：听到声音，记下位置前去调查
func (m *Monster) hear(tx, ty int) {
	m.hasTarget = true
	m.tgtX, m.tgtY = tx, ty
	if m.state != StateChase {
		m.state = StateInvestigate
	}
	m.repath = 0 // 立刻重算寻路场
}

func (m *Monster) calm() {
	m.state = StateWander
	m.hasTarget = false
}

func (m *Monster) update(w *World, px, py float32) {
	mtx, mty := m.tile()
	ptx, pty := int((px+6)/tileSize), int((py+6)/tileSize)
	d := math.Hypot(float64(mtx-ptx), float64(mty-pty))

	// 近距离直接感知到玩家- 追击；追丢-降级为调查
	if d <= 5 {
		m.state = StateChase
	} else if m.state == StateChase && d > 9 {
		m.state = StateInvestigate
	}
	speed := float32(1.5) //比玩家2.5慢，给逃脱空间
	switch m.state {
	case StateChase:
		m.tgtX, m.tgtY, m.hasTarget = ptx, pty, true
		if m.repath > 10 {
			m.repath = 10 //追击时寻路场刷新更勤
		}
		m.moveByField(w, speed)
	case StateInvestigate:
		if !m.moveByField(w, speed*0.8) {
			m.state = StateWander
			m.hasTarget = false
		}
	default:
		m.wander(w, speed*0.5)
	}
	if m.repath > 0 {
		m.repath--
	}
	if m.pingIn > 0 {
		m.pingIn--
	}
}

func (m *Monster) moveByField(w *World, speed float32) bool {
	if !m.hasTarget {
		return false
	}
	mtx, mty := m.tile()
	if m.repath <= 0 {
		w.bfsField(m.tgtX, m.tgtY, &m.field)
		m.repath = 30
	}
	v := m.field[mtx][mty]
	if v == 0 || v == 1 {
		return false
	}
	// 贪心走向field值最小的邻居
	bx, by, best := mtx, mty, v
	for _, d := range dirs4 {
		nx, ny := mtx+d[0], mty+d[1]
		if !w.inBounds(nx, ny) || w.walls[nx][ny] {
			continue
		}
		if fv := m.field[nx][ny]; fv > 0 && fv < best {
			best, bx, by = fv, nx, ny
		}
	}
	if bx == mtx && by == mty {
		return false
	}
	m.moveToward(float32(bx*tileSize+2), float32(by*tileSize+2), speed)
	return true
}

func (m *Monster) moveToward(cx, cy, speed float32) {
	dx, dy := cx-m.x, cy-m.y
	d := float32(math.Hypot(float64(dx), float64(dy)))
	if d <= speed {
		m.x, m.y = cx, cy
		return
	}
	m.x += dx / d * speed
	m.y += dy / d * speed
}

func (m *Monster) wander(w *World, speed float32) {
	m.wTimer--
	reached := math.Abs(float64(m.wX-m.x)) < 2 && math.Abs(float64(m.wY-m.y)) < 2
	if m.wTimer <= 0 || reached {
		mtx, mty := m.tile()
		var opts [][2]int
		for _, d := range dirs4 {
			nx, ny := mtx+d[0], mty+d[1]
			if w.inBounds(nx, ny) && !w.walls[nx][ny] {
				opts = append(opts, [2]int{nx, ny})
			}
		}
		if len(opts) > 0 {
			c := opts[rand.Intn(len(opts))]
			m.wX = float32(c[0]*tileSize + 2)
			m.wY = float32(c[1]*tileSize + 2)
			m.wTimer = 90 + rand.Intn(120)
		}
	}
	m.moveToward(m.wX, m.wY, speed)
}

func (m *Monster) draw(screen *ebiten.Image, w *World) {
	l := w.light[int(m.x+6)/tileSize][int(m.y+6)/tileSize]
	alpha := 0.25 + 0.75*l // 做弱发光，黑暗中隐约可见，被照亮时清晰
	c := color.RGBA{185, 60, 60, uint8(255 * alpha)}
	if m.state == StateInvestigate {
		c = color.RGBA{210, 105, 55, uint8(255 * alpha)}
	} else if m.state == StateChase {
		c = color.RGBA{235, 65, 55, uint8(255 * alpha)}
	}
	vector.FillRect(screen, m.x, m.y, 12, 12, c, false)
	// 眼睛：黑暗中你唯一能辨认的东西
	ec := color.RGBA{255, 238, 225, uint8(220 * alpha)}
	vector.FillRect(screen, m.x+2, m.y+3, 3, 3, ec, false)
	vector.FillRect(screen, m.x+7, m.y+3, 3, 3, ec, false)

}
