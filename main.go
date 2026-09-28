package main

import (
	"image/color"
	"log"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	tileSize = 16
	mapW     = 60
	mapH     = 40
	screenW  = mapW * tileSize //960
	screenH  = mapH * tileSize //640
	maxDist  = 20              //声呐传播距离（格）

)

type Game struct {
	walls [mapW][mapH]bool
	light [mapW][mapH]float32 //每个格子的可见亮度，随时间衰减

	// BFS 波传播状态
	dist     [mapW][mapH]int // 0=未访问，>0=距声源距离
	wave     [][2]int        //当前波前
	pulsing  bool
	curDist  int
	cooldown int //脉冲冷却 (tick)

	px, py float32 //玩家位置(像素，左上)
}

// 程序生成：元胞自动机生成洞穴

func newCave() [mapW][mapH]bool {
	var g [mapW][mapH]bool
	for x := 0; x < mapW; x++ {
		for y := 0; y < mapH; y++ {
			if x == 0 || y == 0 || x == mapW-1 || y == mapH-1 {
				g[x][y] = true // 边界必须是墙
				continue
			}
			g[x][y] = rand.Float32() < 0.45
		}
	}

	// 平滑5轮: >=5 个墙邻居变强，<=3 个变空，保持原有
	for i := 0; i < 5; i++ {
		var ng = g
		for x := 1; x < mapW-1; x++ {
			for y := 1; y < mapH-1; y++ {
				walls := 0
				for dx := -1; dx <= 1; dx++ {
					for dy := -1; dy <= 1; dy++ {
						if dx == 0 && dy == 0 {
							continue
						}
						if g[x+dx][y+dy] {
							walls++
						}
					}
				}
				if walls >= 5 {
					ng[x][y] = true
				} else if walls <= 3 {
					ng[x][y] = false
				}
			}
		}
		g = ng
	}
	return g
}

func (g *Game) spawn() {
	// 从中心向外找第一个空格
	for r := 0; r < 30; r++ {
		for x := mapW/2 - r; x <= mapW/2+r; x++ {
			for y := mapH/2 - r; y <= mapH/2+r; y++ {
				if x > 0 && y > 0 && x < mapW-1 && y < mapH-1 && !g.walls[x][y] {
					g.px = float32(x*tileSize + 2)
					g.py = float32(y*tileSize + 2)
					return
				}
			}
		}
	}
}

// 声呐: BFS波传播。波沿空腔传播，墙壁只被摸到显性，不穿透

func (g *Game) firePulse() {
	g.dist = [mapW][mapH]int{}
	sx, sy := int(g.px+6)/tileSize, int(g.py+6)/tileSize
	g.dist[sx][sy] = 1
	g.light[sx][sy] = 1
	g.wave = append(g.wave[:0], [2]int{sx, sy})
	g.curDist = 1
	g.pulsing = true
}

func (g *Game) stepWave() {
	if !g.pulsing {
		return
	}
	var next [][2]int
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	brightness := 1.0 - float32(g.curDist)/float32(maxDist+1)
	for _, c := range g.wave {
		for _, d := range dirs {
			nx, ny := c[0]+d[0], c[1]+d[1]
			if nx < 0 || ny < 0 || nx >= mapW || ny >= mapH || g.dist[nx][ny] > 0 {
				continue
			}
			g.dist[nx][ny] = g.curDist + 1
			if brightness > g.light[nx][ny] {
				g.light[nx][ny] = brightness
			}
			if !g.walls[nx][ny] {
				next = append(next, [2]int{nx, ny}) // 空格:继续传播
			}
			// 墙: 显形但不入队，波到此为止
		}
	}
	g.curDist++
	if len(next) == 0 || g.curDist > maxDist {
		g.pulsing = false
		return
	}
	g.wave = next
}

// 移动与碰撞:按轴分离，避免卡墙角
func (g *Game) collide(px, py float32) bool {
	x0, y0 := int(px)/tileSize, int(py)/tileSize
	x1, y1 := int(px+11)/tileSize, int(py+11)/tileSize
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if g.walls[x][y] {
				return true
			}
		}
	}
	return false
}

func (g *Game) tryMove(dx, dy float32) {
	if dx != 0 && !g.collide(g.px+dx, g.py) {
		g.px += dx
	}
	if dy != 0 && !g.collide(g.px, g.py+dy) {
		g.py += dy
	}
}

// 游戏主循环三件套
func (g *Game) Update() error {
	const sp = 2.5
	var dx, dy float32
	if ebiten.IsKeyPressed(ebiten.KeyW) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		dy -= sp
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		dy += sp
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		dx -= sp
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		dx += sp
	}
	g.tryMove(dx, dy)

	if g.cooldown > 0 {
		g.cooldown--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.cooldown == 0 {
		g.firePulse()
		g.cooldown = 30 //0.5秒冷却
	}
	g.stepWave()

	// 亮度衰减，玩家脚下保持常亮
	for x := 0; x < mapW; x++ {
		for y := 0; y < mapH; y++ {
			g.light[x][y] *= 0.96
		}
	}
	g.light[int(g.px+6)/tileSize][int(g.py+6)/tileSize] = 1
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{6, 8, 14, 255})
	for x := 0; x < mapW; x++ {
		for y := 0; y < mapH; y++ {
			l := g.light[x][y]
			if l < 0.03 {
				continue
			}
			fx, fy := float32(x*tileSize), float32(y*tileSize)
			if g.walls[x][y] {
				a := uint8(30 + 210*l)
				vector.DrawFilledRect(screen, fx, fy, tileSize, tileSize, color.RGBA{110, 195, 215, a}, false)
			} else {
				a := uint8(70 * l)
				vector.DrawFilledRect(screen, fx, fy, tileSize, tileSize, color.RGBA{32, 42, 64, a}, false)
			}
		}
	}
	vector.DrawFilledRect(screen, g.px, g.py, 12, 12, color.RGBA{255, 214, 110, 255}, false)
	ebitenutil.DebugPrint(screen, "WASD 移动·空格 声呐脉冲")
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenW, screenH
}

func main() {
	g := &Game{}
	g.walls = newCave()
	g.spawn()
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Sonar Abyss - M1")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
