package main

import "math/rand"

const (
	tileSize = 16
	mapW     = 60
	mapH     = 40
	screenW  = mapW * tileSize //960
	screenH  = mapH * tileSize
	maxDist  = 20
)

var dirs4 = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

type World struct {
	walls [mapW][mapH]bool
	light [mapW][mapH]float32

	// BFS 声呐波传播
	dist     [mapW][mapH]int
	wave     [][2]int
	pulsing  bool
	curDist  int
	pulseMax int
}

func newWorld() *World {
	w := &World{}
	// 元胞自动机生成洞穴
	for x := 0; x < mapW; x++ {
		for y := 0; y < mapH; y++ {
			if x == 0 || y == 0 || x == mapW-1 || y == mapH-1 {
				w.walls[x][y] = true
				continue
			}
			w.walls[x][y] = rand.Float32() < 0.45
		}
	}
	for i := 0; i < 5; i++ {
		var ng = w.walls
		for x := 1; x < mapW-1; x++ {
			for y := 1; y < mapH-1; y++ {
				n := 0
				for dx := -1; dx <= 1; dx++ {
					for dy := -1; dy <= 1; dy++ {
						if dx == 0 && dy == 0 {
							continue
						}
						if w.walls[x+dx][y+dy] {
							n++
						}
					}
				}
				if n >= 5 {
					ng[x][y] = true
				} else if n <= 3 {
					ng[x][y] = false
				}
			}
		}
		w.walls = ng
	}
	return w
}

func (w *World) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < mapW && y < mapH
}

// 声呐：BFS波传播，波沿空腔传播，墙只显形不穿透
func (w *World) firePulse(centerX, centerY float32, radius int) {
	w.dist = [mapW][mapH]int{}
	sx, sy := int(centerX)/tileSize, int(centerY)/tileSize
	w.dist[sx][sy] = 1
	w.light[sx][sy] = 1
	w.wave = append(w.wave[:0], [2]int{sx, sy})
	w.curDist = 1
	w.pulseMax = radius
	w.pulsing = true
}

func (w *World) stepWave() {
	if !w.pulsing {
		return
	}

	var next [][2]int
	brightness := 1.0 - float32(w.curDist)/float32(w.pulseMax+1)
	for _, c := range w.wave {
		for _, d := range dirs4 {
			nx, ny := c[0]+d[0], c[1]+d[1]
			if !w.inBounds(nx, ny) || w.dist[nx][ny] > 0 {
				continue
			}
			w.dist[nx][ny] = w.curDist + 1
			if brightness > w.light[nx][ny] {
				w.light[nx][ny] = brightness
			}
			if !w.walls[nx][ny] {
				next = append(next, [2]int{nx, ny})
			}
		}
	}
	w.curDist++
	if len(next) == 0 || w.curDist > w.pulseMax {
		w.pulsing = false
		return
	}
	w.wave = next
}

func (w *World) decay() {
	for x := 0; x < mapW; x++ {
		for y := 0; y < mapH; y++ {
			w.light[x][y] *= 0.96
		}
	}
}

func (w *World) lightAtPlayer(px, py float32) {
	w.light[int(px+6)/tileSize][int(py+6)/tileSize] = 1
}

// 碰撞

func (w *World) collide(px, py float32) bool {
	x0, y0 := int(px)/tileSize, int(py)/tileSize
	x1, y1 := int(px+11)/tileSize, int(py+11)/tileSize
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if w.walls[x][y] {
				return true
			}
		}
	}
	return false
}

func (w *World) findSpawn(px, py *float32) {
	for r := 0; r < 30; r++ {
		for x := mapW/2 - r; x <= mapW/2+r; x++ {
			for y := mapH/2 - r; y <= mapH/2+r; y++ {
				if w.inBounds(x, y) && !w.walls[x][y] {
					*px = float32(x*tileSize + 2)
					*py = float32(y*tileSize + 2)
					return
				}
			}
		}
	}
}

func (w *World) randomOpenTileFarFrom(px, py float32, minTiles int) (float32, float32) {
	ptx, pty := int(px)/tileSize, int(py)/tileSize
	for i := 0; i < 300; i++ {
		x := 1 + rand.Intn(mapW-2)
		y := 1 + rand.Intn(mapH-2)
		if w.walls[x][y] {
			continue
		}
		dx, dy := float64(x-ptx), float64(y-pty)
		if dx*dx+dy*dy < float64(minTiles*minTiles) {
			continue
		}
		return float32(x*tileSize + 2), float32(y*tileSize + 2)
	}
	return tileSize * 2, tileSize * 2
}

// BFS寻路场：从目标点洪水漫灌，怪物沿field值递减的方向走
func (w *World) bfsField(tx, ty int, field *[mapW][mapH]int) {
	*field = [mapW][mapH]int{}
	if w.walls[tx][ty] {
		return
	}
	field[tx][ty] = 1
	q := make([][2]int, 0, 512)
	q = append(q, [2]int{tx, ty})
	for h := 0; h < len(q); h++ {
		c := q[h]
		for _, d := range dirs4 {
			nx, ny := c[0]+d[0], c[1]+d[1]
			if !w.inBounds(nx, ny) || field[nx][ny] != 0 || w.walls[nx][ny] {
				continue
			}
			field[nx][ny] = field[c[0]][c[1]] + 1
			q = append(q, [2]int{nx, ny})
		}
	}
}
