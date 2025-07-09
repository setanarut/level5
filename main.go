package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"math/rand/v2"
	"strconv"

	"github.com/setanarut/coll"
	"github.com/setanarut/maze"
	"github.com/setanarut/v"
	"golang.org/x/text/language"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/mlange-42/ark/ecs"
)

//go:embed font/*
var fs embed.FS

var (
	screenSize             = image.Point{854, 480}
	screenSizeF            = v.Vec{854, 480}
	screenCenter           = screenSizeF.Scale(0.5)
	maz                    = maze.NewMaze[uint8](9, 5, 8, 1)
	collider               = coll.NewCollider(maz.Grid, 9, 9)
	world        ecs.World = ecs.NewWorld()
	mapPlayer              = ecs.NewMap[Player](&world)
	mapBullet              = ecs.NewMap[Bullet](&world)
	mapGoal                = ecs.NewMap[Goal](&world)
	filterGoal             = ecs.NewFilter1[Goal](&world)
	filterPlayer           = ecs.NewFilter1[Player](&world)
	filterBall             = ecs.NewFilter1[Bullet](&world)
	ballSpeed              = 0.7
	playerSpeed            = 0.7
	tick                   = 0
	playerColor            = color.RGBA{255, 255, 255, 255}
	bulletColor            = color.RGBA{255, 128, 255, 255}
	goalColor              = color.RGBA{0, 255, 0, 255}

	drawOffset = v.Vec{}

	ballRandomSource = rand.New(rand.NewPCG(0, 0))
	font             = LoadFontFromFS("font/arkpixel10.ttf", 128, fs)
	fontSmall        = LoadFontFromFS("font/arkpixel10.ttf", 20, fs)
	textOpt          = &text.DrawOptions{}
	textOptSmall     = &text.DrawOptions{}
)

var deathCount = 0
var gameState = 3

var (
	playing   = 0
	dying     = 1
	paused    = 2
	nextLevel = 3
	gameOver  = 4
	waitState = 5
)

var (
	levelSeed uint64 = 1
)

func init() {
	textOpt.LineSpacing = 128
	textOptSmall.LineSpacing = 20
	mazeSize := maz.Size().Mul(collider.CellSize.X)
	drawOffset.X = float64(screenSize.X-mazeSize.X) * 0.5
	drawOffset.Y = float64(screenSize.Y-mazeSize.Y) * 0.5
	mapPlayer.NewEntity(&Player{
		Box: coll.AABB{
			Pos:  tileToWorld(0, 0),
			Half: v.Vec{4, 4},
		},
	})
}

type Bullet struct {
	Box coll.AABB
	Vel v.Vec
}
type Player struct {
	Box coll.AABB
	Vel v.Vec
}

type Goal struct {
	Box coll.AABB
}

type Game struct {
}

func (g *Game) Update() error {
	switch gameState {
	case playing:
		playState()
	case dying:
		dieState()
	case nextLevel:
		nextLevelState()
	case waitState:
		wait()
	case paused:
		pauseState()
	case gameOver:
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			levelSeed = 1
			gameState = nextLevel
		}
	}
	return nil
}

func dieState() {
	if (tick/8)%2 == 0 {
		playerColor = color.RGBA{255, 0, 0, 255} // 30 kare kırmızı
		bulletColor = color.RGBA{255, 0, 0, 255} // 30 kare kırmızı
	} else {
		playerColor = color.RGBA{255, 255, 255, 255} // 30 kare beyaz
		bulletColor = color.RGBA{255, 255, 255, 255} // 30 kare beyaz
	}
	tick++

	// --- RESET GAME ---
	if tick == 60 {
		deathCount++
		tick = 0
		gameState = playing

		// maz.Generate(levelSeed, 0) // level seed
		ballRandomSource = rand.New(rand.NewPCG(levelSeed, 0))

		world.RemoveEntities(filterBall.Batch(), nil)

		// reset player position
		queryPlayer := filterPlayer.Query()
		queryPlayer.Next()
		player := queryPlayer.Get()
		// x, y := rand.IntN(maz.Cols), rand.IntN(maz.Rows)
		x, y := 0, 0
		player.Box.Pos = tileToWorld(x, y)
		queryPlayer.Close()

		// Spawn bullets
		for i := range maz.Cols {
			for j := range maz.Rows {
				if i == 0 && j == 0 {
					continue
				}

				randomAngle := ballRandomSource.Float64() * 2 * math.Pi
				mapBullet.NewEntity(&Bullet{
					Box: coll.AABB{
						Pos:  tileToWorld(i, j),
						Half: v.Vec{2, 2},
					},
					Vel: v.FromAngle(randomAngle).Scale(ballSpeed),
				})
			}
		}
		// reset Colors
		playerColor = color.RGBA{255, 255, 255, 255}
		bulletColor = color.RGBA{255, 128, 255, 255}
	}
}

func nextLevelState() {

	if levelSeed == 6 {
		gameState = gameOver
		return
	}

	if tick == 0 {
		ballSpeed += 0.3
		playerSpeed -= 0.1
		ballRandomSource = rand.New(rand.NewPCG(levelSeed, 0))
		maz.Generate(levelSeed, 0) // level seed
		world.RemoveEntities(filterGoal.Batch(), nil)
		world.RemoveEntities(filterBall.Batch(), nil)

		// Spawn bullets
		for i := range maz.Cols {
			for j := range maz.Rows {
				if i == 0 && j == 0 {
					continue
				}

				randomAngle := ballRandomSource.Float64() * 2 * math.Pi
				mapBullet.NewEntity(&Bullet{
					Box: coll.AABB{
						Pos:  tileToWorld(i, j),
						Half: v.Vec{2, 2},
					},
					Vel: v.FromAngle(randomAngle).Scale(ballSpeed),
				})
			}
		}

		// reset player position
		queryPlayer := filterPlayer.Query()
		queryPlayer.Next()
		player := queryPlayer.Get()
		x, y := 0, 0
		player.Box.Pos = tileToWorld(x, y)
		queryPlayer.Close()

		// Spawn goal
		mapGoal.NewEntity(&Goal{
			Box: coll.AABB{
				Pos:  tileToWorld(maz.Cols-1, maz.Rows-1),
				Half: v.Vec{4, 4},
			},
		})

	}

	// --- Next Level ---
	if tick > 120 {
		tick = 0
		gameState = waitState
	}
	tick++

}

func pauseState() {
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		gameState = playing
	}
}

var waitTick = 0
var waitCountdown = 0

func wait() {
	waitCountdown = 3 - (waitTick / 60)
	waitTick++
	if waitTick > 180 {
		waitTick = 0
		gameState = playing
		waitCountdown = 0
	}
}

func playState() {

	queryPlayer := filterPlayer.Query()
	for queryPlayer.Next() {
		player := queryPlayer.Get()
		player.Vel = Axis().Unit().Scale(playerSpeed)
		player.Box.Pos = player.Box.Pos.Add(player.Vel)

		collider.Collide(
			player.Box,
			player.Vel,
			func(hti []coll.HitTileInfo, dx, dy float64) {
				if len(hti) > 0 {
					gameState = dying
					tick = 0
				}
				player.Box.Pos = player.Box.Pos.Add(v.Vec{dx, dy})
			},
		)

		// inner Bullet query
		queryBullet := filterBall.Query()
		for queryBullet.Next() {
			bullet := queryBullet.Get()
			collider.Collide(
				bullet.Box,
				bullet.Vel,
				func(hti []coll.HitTileInfo, dx, dy float64) {
					if len(hti) > 0 {
						tick = 0
						// Sadece ilk çarpışmanın normalini kullan
						bullet.Vel = bullet.Vel.Reflect(hti[0].Normal)
					}
					// Sadece izin verilen mesafe kadar ilerle
					bullet.Box.Pos = bullet.Box.Pos.Add(v.Vec{dx, dy})
				},
			)
			if coll.Overlap(&player.Box, &bullet.Box, nil) {
				// if coll.OverlapSweep2(&player.Box, &bullet.Box, player.Vel, bullet.Vel, hitInf) {
				gameState = dying
			}
		}

		// innter Goal query
		queryGoal := filterGoal.Query()
		for queryGoal.Next() {
			goal := queryGoal.Get()

			if coll.Overlap(&goal.Box, &player.Box, nil) {
				tick = 0
				levelSeed++
				if levelSeed == 6 {
					gameState = gameOver
				}
				gameState = nextLevel
			}
		}

	}

}

func (g *Game) Draw(s *ebiten.Image) {

	switch gameState {
	case gameOver:
		// pos := screenSize.Div(2)
		// ebitenutil.DebugPrintAt(s, "END GAME \n"+fmt.Sprintf("Deaths %v", deathCount), pos.X-10, pos.Y)
		textOptSmall.GeoM.Reset()
		textOptSmall.GeoM.Translate(350, 200)
		text.Draw(s, fmt.Sprintf("GAME OVER\nDeaths %v\nPress Space to restart", deathCount), fontSmall, textOptSmall)

	case waitState:
		drawTileMap(s)
		drawPlayer(s)
		drawBullets(s)
		drawGoal(s)

		textOpt.GeoM.Reset()
		textOpt.GeoM.Translate(0, -64)
		textOpt.GeoM.Translate(screenCenter.X, screenCenter.Y)
		textOpt.GeoM.Translate(-30, -45)
		// textOpt.ColorScale.Scale(1, 1, 0, 1)
		if waitCountdown > 0 {
			text.Draw(s, strconv.Itoa(waitCountdown), font, textOpt)
		}

	case nextLevel:
		if levelSeed != 6 {
			textOpt.GeoM.Reset()
			textOpt.GeoM.Translate(-200, -64)
			textOpt.GeoM.Translate(screenCenter.X, screenCenter.Y)
			textOpt.GeoM.Translate(-32, -50)
			text.Draw(s, fmt.Sprintf("LEVEL %d", levelSeed), font, textOpt)
		}

	default:
		if levelSeed != 6 {
			drawTileMap(s)
			drawPlayer(s)
			drawBullets(s)
			drawGoal(s)
			textOptSmall.GeoM.Reset()
			textOptSmall.GeoM.Translate(screenCenter.X-100, 0)
			text.Draw(s, fmt.Sprintf("LEVEL %d", levelSeed), fontSmall, textOptSmall)
			textOptSmall.GeoM.Translate(100, 0)
			text.Draw(s, fmt.Sprintf("DEATHS %d", deathCount), fontSmall, textOptSmall)
		}
	}

}

func drawBullets(screen *ebiten.Image) {
	q := filterBall.Query()
	for q.Next() {
		bullet := q.Get()
		drawAABB(screen, &bullet.Box, bulletColor)
	}
}
func drawGoal(screen *ebiten.Image) {
	q := filterGoal.Query()
	for q.Next() {
		goal := q.Get()
		drawAABB(screen, &goal.Box, goalColor)
	}
}

func drawPlayer(screen *ebiten.Image) {
	queryPlayer := filterPlayer.Query()
	for queryPlayer.Next() {
		player := queryPlayer.Get()
		drawAABB(screen, &player.Box, playerColor)
	}
}

func drawTileMap(screen *ebiten.Image) {
	xo := float32(drawOffset.X)
	yo := float32(drawOffset.Y)

	for y := range maz.Grid {
		for x := range maz.Grid[y] {
			if maz.Grid[y][x] != 0 {
				vector.DrawFilledRect(screen,
					float32((x*collider.CellSize.X))+xo,
					float32((y*collider.CellSize.Y))+yo,
					float32(collider.CellSize.X),
					float32(collider.CellSize.Y),
					color.Gray{Y: 128},
					false)
			}
		}
	}
}

func drawAABB(s *ebiten.Image, aabb *coll.AABB, clr color.Color) {
	vector.DrawFilledRect(
		s,
		float32((aabb.Pos.X-aabb.Half.X)+drawOffset.X),
		float32((aabb.Pos.Y-aabb.Half.Y)+drawOffset.Y),
		float32(aabb.Half.X*2),
		float32(aabb.Half.Y*2),
		clr,
		false,
	)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenSize.X, screenSize.Y
}

func main() {
	rgo := &ebiten.RunGameOptions{
		DisableHiDPI: true,
	}
	ebiten.SetWindowSize(screenSize.X, screenSize.Y)
	if err := ebiten.RunGameWithOptions(&Game{}, rgo); err != nil {
		log.Fatal(err)
	}
}

func Axis() (vel v.Vec) {
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		vel.Y -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		vel.Y += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		vel.X -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		vel.X += 1
	}
	return
}

func tileToWorld(x, y int) v.Vec {
	cellX, cellY := float64(x+1), float64(y+1)
	actualCellSize := float64(maz.CellSize+1) * float64(collider.CellSize.X)
	half := actualCellSize / 2
	return v.Vec{(actualCellSize * cellX) - half, (actualCellSize * cellY) - half}
}

func LoadFontFromFS(file string, size float64, fs embed.FS) *text.GoTextFace {
	f, err := fs.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(f))
	if err != nil {
		log.Fatal(err)
	}
	gtf := &text.GoTextFace{
		Source:   src,
		Size:     size,
		Language: language.English,
	}
	return gtf
}
