package main

import (
	"image/color"
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Game struct {
	world    *World
	sb       *SoundBoard
	monsters []*Monster

	px, py         float32
	spawnX, spawnY float32
	cooldown       int
	stepTimer      int
	alive          bool
	respawnIn      int
}

func newGame() *Game {
	g := &Game{world: newWorld(), sb: newSoundBoard(), alive: true}
	g.world.findSpawn(&g.px, &g.py)
	g.spawnX, g.spawnY = g.px, g.py
	for i := 0; i < 6; i++ {
		x, y := g.world.randomOpenTileFarFrom(g.px, g.py, 15)
		g.monsters = append(g.monsters, newMonster(x, y))
	}
	return g
}

// alertMonsters：把"声音事件"通知给定半径内的怪物
func (g *Game) alertMonsters(radius float64) {
	ptx, pty := int((g.px+6)/tileSize), int((g.py+6)/tileSize)
	for _, m := range g.monsters {
		mtx, mty := m.tile()
		d := math.Hypot(float64(mtx-ptx), float64(mty-pty))
		if d <= radius {
			before := m.state
			m.hear(ptx, pty)
			if before == StateWander {
				g.sb.playVol(g.sb.alert, g.volAt(m))
			}
		}
	}
}

func (g *Game) volAt(m *Monster) float64 {
	d := math.Hypot(float64(m.x-g.px), float64(m.y-g.py)) / tileSize
	v := 1 - d/32
	return math.Max(0.05, math.Min(1, v)) * 0.85
}
func (g *Game) die() {
	if !g.alive {
		return
	}
	g.alive = false
	g.respawnIn = 150
	g.sb.playVol(g.sb.death, 0.9)
}

func (g *Game) tryMove(dx, dy float32) {
	if dx != 0 && !g.world.collide(g.px+dx, g.py) {
		g.px += dx
	}
	if dy != 0 && !g.world.collide(g.px, g.py+dy) {
		g.py += dy
	}
}
func (g *Game) Update() error {
	g.world.decay()
	if !g.alive {
		g.respawnIn--
		if g.respawnIn <= 0 {
			g.alive = true
			g.px, g.py = g.spawnX, g.spawnY
			for _, m := range g.monsters {
				m.calm()
			}
		}
		return nil
	}
	var dx, dy float32
	moving := false
	const sp = 2.5
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		dy -= sp
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		dy += sp
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		dx -= sp
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		dx += sp
	}
	if dx != 0 || dy != 0 {
		moving = true
	}
	g.tryMove(dx, dy)
	// 脚步声：移动每18tick响一次，半径5格内的怪物听的到
	if moving {
		g.stepTimer++
		if g.stepTimer >= 18 {
			g.stepTimer = 0
			g.sb.playVol(g.sb.step, 0.25)
			g.alertMonsters(5)
		}
	}
	// 声呐脉冲：照亮地图，但半径 26 格内的怪物全部听见
	if g.cooldown > 0 {
		g.cooldown--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.cooldown == 0 {
		g.cooldown = 30
		g.sb.playVol(g.sb.ping, 0.7)
		g.world.firePulse(g.px+6, g.py+6, maxDist)
		g.alertMonsters(26)
	}
	g.world.stepWave()
	g.world.lightAtPlayer(g.px, g.py)

	// 怪物：AI 更新 + 自身脉冲 + 接触判定
	for _, m := range g.monsters {
		m.update(g.world, g.px, g.py)
		if m.pingIn <= 0 {
			m.pingIn = 200 + rand.Intn(200)
			g.world.firePulse(m.x+6, m.y+6, 14)
			g.sb.playVol(g.sb.mPing, g.volAt(m))
		}
		if math.Abs(float64(m.x-g.px)) < 11 && math.Abs(float64(m.y-g.py)) < 11 {
			g.die()
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{6, 8, 14, 255})
	for x := 0; x < mapW; x++ {
		for y := 0; y < mapH; y++ {
			l := g.world.light[x][y]
			if l < 0.03 {
				continue
			}
			fx, fy := float32(x*tileSize), float32(y*tileSize)
			if g.world.walls[x][y] {
				a := uint8(30 + 210*l)
				vector.FillRect(screen, fx, fy, tileSize, tileSize, color.RGBA{110, 195, 215, a}, false)
			} else {
				a := uint8(70 * l)
				vector.FillRect(screen, fx, fy, tileSize, tileSize, color.RGBA{32, 42, 64, a}, false)
			}
		}
	}
	for _, m := range g.monsters {
		m.draw(screen, g.world)
	}
	if g.alive {
		vector.FillRect(screen, g.px, g.py, 12, 12, color.RGBA{255, 214, 110, 255}, false)
	}
	ebitenutil.DebugPrint(screen, "WASD 移动 · 空格 声呐脉冲 · 声音会引来东西")
	if !g.alive {
		ebitenutil.DebugPrintAt(screen, "你被抓住了……即将重生于入口", screenW/2-130, screenH/2)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}

func main() {
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Sonar Abyss - M2")
	if err := ebiten.RunGame(newGame()); err != nil {
		log.Fatal(err)
	}
}
