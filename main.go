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

type GameState int

const (
	StateTittle  GameState = iota // 标题画面
	StatePlaying                  //游戏中
	StateDead                     //死亡结算
	StateWin                      //掏出深渊
)

const (
	oxygenMaxTicks = 90 * 60
	crystalBonus   = 12 * 60
)

type Game struct {
	world    *World
	sb       *SoundBoard
	monsters []*Monster

	state        GameState
	floor        int
	score        int
	crystalCount int
	oxygen       float32
	deathReason  string

	px, py float32
	//	spawnX, spawnY float32
	cooldown   int
	stepTimer  int
	tick       int
	heartTimer int
	//alive     bool
	//respawnIn int
}

func newGame() *Game {
	g := &Game{sb: newSoundBoard(), state: StateTittle}
	g.startNewRun()

	return g
}

func (g *Game) startNewRun() {
	g.floor = 1
	g.score = 0
	g.crystalCount = 0
	g.loadFloor()
}

func (g *Game) loadFloor() {
	g.world = newWorld(g.floor)
	g.px, g.py = g.world.spawnX, g.world.spawnY
	g.oxygen = oxygenMaxTicks
	g.monsters = g.monsters[:0]
	n := 2 + g.floor
	for i := 0; i < n; i++ {
		x, y := g.world.randomOpenTileFarFrom(g.px, g.py, 12)
		speed := 1.4 + 0.1*float32(g.floor)
		if speed > 2.2 {
			speed = 2.2
		}
		g.monsters = append(g.monsters, newMonster(x, y, speed))
	}
}

func (g *Game) gameOver(reason string) {
	g.state = StateDead
	g.deathReason = reason
	g.sb.playVol(g.sb.death, 0.9)
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

func (g *Game) tryMove(dx, dy float32) {
	if dx != 0 && !g.world.collide(g.px+dx, g.py) {
		g.px += dx
	}
	if dy != 0 && !g.world.collide(g.px, g.py+dy) {
		g.py += dy
	}
}
func (g *Game) updatePlaying() {
	g.world.decay()

	// ---- 氧气经济：持续消耗，水晶补充，低氧心跳警告 ----
	g.oxygen--
	if g.oxygen <= 0 {
		g.gameOver("氧气耗尽，你永远沉入了黑暗")
		return
	}
	if g.oxygen < 20*60 {
		g.heartTimer--
		if g.heartTimer <= 0 {
			g.heartTimer = 28
			g.sb.playVol(g.sb.heart, 0.5)
		}
	}

	// ---- 移动与脚步 ----
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

	if moving {
		g.stepTimer++
		if g.stepTimer >= 18 {
			g.stepTimer = 0
			g.sb.playVol(g.sb.step, 0.25)
			g.alertMonsters(5)
		}
	}

	// ---- 声呐脉冲 ----
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

	// ---- 拾取水晶 ----
	if g.world.takeCrystalAt(g.px, g.py) {
		g.crystalCount++
		g.score += 10
		g.oxygen += crystalBonus
		if g.oxygen > oxygenMaxTicks {
			g.oxygen = oxygenMaxTicks
		}
		g.sb.playVol(g.sb.pickup, 0.6)
	}

	// ---- 出口：下潜或逃脱 ----
	if g.world.atExit(g.px, g.py) {
		g.score += 50
		g.sb.playVol(g.sb.descend, 0.8)
		if g.floor >= maxFloor {
			g.state = StateWin
			return
		}
		g.floor++
		g.loadFloor()
		return
	}

	// ---- 怪物 ----
	for _, m := range g.monsters {
		m.update(g.world, g.px, g.py)
		if m.pingIn <= 0 {
			m.pingIn = 200 + rand.Intn(200)
			g.world.firePulse(m.x+6, m.y+6, 14)
			g.sb.playVol(g.sb.mPing, g.volAt(m))
		}
		if math.Abs(float64(m.x-g.px)) < 11 && math.Abs(float64(m.y-g.py)) < 11 {
			g.gameOver("你被深渊生物吞噬了")
			return
		}
	}
}
func (g *Game) Update() error {
	g.tick++
	switch g.state {
	case StateTittle:
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.state = StatePlaying
		}
	case StateDead, StateWin:
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.startNewRun()
			g.state = StatePlaying
		}
	case StatePlaying:
		g.updatePlaying()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.state == StateTittle {
		screen.Fill(color.RGBA{6, 8, 14, 255})
		ebitenutil.DebugPrintAt(screen, "声 呐 深 渊", screenW/2-48, screenH/2-70)
		ebitenutil.DebugPrintAt(screen, "你在黑暗的海底深渊中醒来，肉眼只能看见脚下。", screenW/2-170, screenH/2-30)
		ebitenutil.DebugPrintAt(screen, "声呐脉冲能照亮周围——但也会引来深渊中的东西。", screenW/2-172, screenH/2-10)
		ebitenutil.DebugPrintAt(screen, "收集氧气水晶，找到舱门，下潜到第 5 层并活着离开。", screenW/2-178, screenH/2+10)
		ebitenutil.DebugPrintAt(screen, "WASD 移动 · 空格 声呐脉冲", screenW/2-90, screenH/2+50)
		ebitenutil.DebugPrintAt(screen, "按 Enter 开始", screenW/2-44, screenH/2+90)
		return
	}

	screen.Fill(color.RGBA{6, 8, 14, 255})
	// 地形
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
	// 水晶：
	for x := 0; x < mapW; x++ {
		for y := 0; y < mapH; y++ {
			if !g.world.crystals[x][y] {
				continue
			}
			pulse := float32(0.35 + 0.15*math.Sin(float64(g.tick)*0.05+float64(x+y)))
			if l := g.world.light[x][y]; l > pulse {
				pulse = l
			}
			c := color.RGBA{120, 230, 255, uint8(255 * pulse)}
			vector.FillRect(screen, float32(x*tileSize+4), float32(y*tileSize+4), 8, 8, c, false)
		}
	}
	// 出口
	exitPulse := 0.45 + 0.25*math.Sin(float64(g.tick)*0.04)
	vector.FillRect(screen, float32(g.world.exitX*tileSize+2),
		float32(g.world.exitY*tileSize+2), 12, 12, color.RGBA{90, 230, 140, uint8(255 * exitPulse)},
		false)

	for _, m := range g.monsters {
		m.draw(screen, g.world)
	}
	vector.FillRect(screen, g.px, g.py, 12, 12, color.RGBA{255, 214, 110, 255}, false)
	// HUD
	ebitenutil.DebugPrint(screen, "WASD 移动 · 空格 脉冲")
	ratio := g.oxygen / oxygenMaxTicks
	barW := float32(180)
	barC := color.RGBA{90, 200, 120, 255}
	if ratio < 0.5 {
		barC = color.RGBA{220, 190, 80, 255}
	}
	if ratio < 0.2 {
		barC = color.RGBA{220, 80, 70, 255}
	}
	vector.DrawFilledRect(screen, screenW-200, 24, barW, 8, color.RGBA{40, 44, 56, 255}, false)
	vector.DrawFilledRect(screen, screenW-200, 24, barW*ratio, 8, barC, false)
	ebitenutil.DebugPrintAt(screen, "氧气", screenW-200, 6)
	ebitenutil.DebugPrintAt(screen, "第 "+itoa(g.floor)+" 层 · 晶簇 "+itoa(g.crystalCount)+" · 分数 "+itoa(g.score),
		screenW-200, 40)

	// ---- 结算画面 ----
	if g.state == StateDead {
		ebitenutil.DebugPrintAt(screen, g.deathReason, screenW/2-130, screenH/2-50)
		ebitenutil.DebugPrintAt(screen, "到达第 "+itoa(g.floor)+" 层 · 晶簇 "+itoa(g.crystalCount)+" · 分数 "+itoa(g.score),
			screenW/2-140, screenH/2-20)
		ebitenutil.DebugPrintAt(screen, "按 R 再次下潜", screenW/2-50, screenH/2+20)
	}
	if g.state == StateWin {
		ebitenutil.DebugPrintAt(screen, "你逃出了深渊。", screenW/2-50, screenH/2-50)
		ebitenutil.DebugPrintAt(screen, "晶簇 "+itoa(g.crystalCount)+" · 分数 "+itoa(g.score),
			screenW/2-70, screenH/2-20)
		ebitenutil.DebugPrintAt(screen, "按 R 再来一局", screenW/2-50, screenH/2+20)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func main() {
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Sonar Abyss - M3")
	if err := ebiten.RunGame(newGame()); err != nil {
		log.Fatal(err)
	}
}
