package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/pkg/errors"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed click.mp3
var clickMp3 []byte

//go:embed cheer.mp3
var cheerMp3 []byte

func main() {
	g := newGame(7, 6)
	err := g.loadImages()
	if err != nil {
		log.Fatal(err)
	}
	err = g.loadSounds()
	if err != nil {
		fmt.Printf("failed to load sounds err=%v\n", err)
	}
	err = g.loadFonts()
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(g.screenWidth(), g.screenHeight())
	ebiten.SetWindowTitle("4 Gewinnt")

	err = ebiten.RunGame(g)
	if err != nil {
		log.Fatal(err)
	}
}

func printLine(ps []*peg) {
	fmt.Printf("line: ")
	for _, p := range ps {
		fmt.Printf("%s ", p)
	}
	fmt.Println()
}

type peg struct {
	g      *Game
	x, y   int
	player int
}

func (p *peg) String() string {
	return fmt.Sprintf("peg{x=%v y=%v player=%v}", p.x, p.y, p.player)
}

func (p *peg) isIn(others []*peg) bool {
	for _, o := range others {
		if o.x == p.x && o.y == p.y {
			return true
		}
	}
	return false
}

func (p *peg) neighbor(d direction) *peg {
	nextX, nextY, onBoard := p.g.nextPos(p.x, p.y, d)
	if !onBoard {
		return nil
	}
	return p.g.pegs[nextX][nextY]
}

func (p *peg) hasFour() ([]*peg, bool) {
	checkLine := func(directionA, directionB direction) ([]*peg, bool) {
		streak := []*peg{p}
		next := p
		for i := 0; i < 4; i++ {
			next = next.neighbor(directionA)
			if next == nil || next.player != p.player {
				break
			}
			streak = append(streak, next)
		}
		next = p
		for i := 0; i < 4; i++ {
			next = next.neighbor(directionB)
			if next == nil || next.player != p.player {
				break
			}
			streak = append(streak, next)
		}

		return streak, len(streak) >= 4
	}

	// \
	line, ok := checkLine(NorthWest, SouthEast)
	if ok {
		return line, ok
	}
	// /
	line, ok = checkLine(SouthWest, NorthEast)
	if ok {
		return line, ok
	}
	// -
	line, ok = checkLine(West, East)
	if ok {
		return line, ok
	}
	// |
	line, ok = checkLine(North, South)
	if ok {
		return line, ok
	}

	return nil, false
}

type button struct {
	label         string
	x, y          int
	width, height int
}

type Game struct {
	pegs            [][]*peg
	pressedTouchIDs []ebiten.TouchID
	releasedKeys    []ebiten.Key

	activePlayer int
	winner       int
	winningLine  []*peg

	isStart bool

	columns, rows int
	blockSize     int

	activeButton *button

	images map[string]*ebiten.Image
	sounds map[string]*audio.Player
	fonts  map[string]font.Face
}

func newGame(columns, rows int) *Game {
	g := &Game{
		blockSize:    90,
		activePlayer: 1,
		pegs:         [][]*peg{},
		images:       map[string]*ebiten.Image{},
		sounds:       map[string]*audio.Player{},
		fonts:        map[string]font.Face{},
	}

	g.columns = columns
	g.rows = rows

	g.pressedTouchIDs = make([]ebiten.TouchID, 0, 48)
	g.releasedKeys = make([]ebiten.Key, 0, 48)
	g.reset()

	return g
}

func (g *Game) loadImages() error {
	empty, _, err := ebitenutil.NewImageFromFile("empty.png")
	if err != nil {
		return errors.Wrap(err, "failed to load empty.png")
	}
	red, _, err := ebitenutil.NewImageFromFile("red.png")
	if err != nil {
		return errors.Wrap(err, "failed to load red.png")
	}
	yellow, _, err := ebitenutil.NewImageFromFile("yellow.png")
	if err != nil {
		return errors.Wrap(err, "failed to load yellow.png")
	}
	pink, _, err := ebitenutil.NewImageFromFile("pink.png")
	if err != nil {
		return errors.Wrap(err, "failed to load yellow.png")
	}

	g.images["empty"] = empty
	g.images["red"] = red
	g.images["yellow"] = yellow
	g.images["pink"] = pink

	return nil
}

func (g *Game) loadFonts() error {
	trueType, err := opentype.Parse(fonts.PressStart2P_ttf)
	if err != nil {
		return errors.Wrap(err, "failed to parse font")
	}

	arcadeFont, err := opentype.NewFace(
		trueType,
		&opentype.FaceOptions{
			Size:    14,
			DPI:     72,
			Hinting: font.HintingFull,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to create new face")
	}

	g.fonts["arcade"] = arcadeFont

	return nil
}

func (g *Game) loadSounds() error {
	audioContext := audio.NewContext(48_000)

	clickStream, err := mp3.DecodeWithoutResampling(bytes.NewReader(clickMp3))
	if err != nil {
		return errors.Wrap(err, "failed to load click.mp3")
	}
	click, err := audioContext.NewPlayer(clickStream)
	if err != nil {
		return errors.Wrap(err, "failed to create click player")
	}

	cheerStream, err := mp3.DecodeWithoutResampling(bytes.NewReader(cheerMp3))
	if err != nil {
		return errors.Wrap(err, "failed to load cheer.mp3")
	}
	cheer, err := audioContext.NewPlayer(cheerStream)
	if err != nil {
		return errors.Wrap(err, "failed to create cheer player")
	}

	g.sounds["click"] = click
	g.sounds["cheer"] = cheer

	return nil
}

func (g *Game) click(x, y int) {
	if g.isStart {
		g.activeButton = nil
		g.isStart = false
	}

	if g.isFinished() && g.isButtonClick(x, y) {
		g.reset()
		return
	}

	col, ok := g.positionToColumn(x, y)
	if !ok {
		col = -1
	}
	fmt.Printf("click x=%v y=%v col=%v\n", x, y, col)
	p := g.addPeg(col)
	if p != nil {
		g.finishTurn()
	}
}

func (g *Game) Update() error {
	g.pressedTouchIDs = g.pressedTouchIDs[:0]
	g.pressedTouchIDs = inpututil.AppendJustReleasedTouchIDs(g.pressedTouchIDs)

	for _, tid := range g.pressedTouchIDs {
		x, y := inpututil.TouchPositionInPreviousTick(tid)
		g.click(x, y)
	}

	g.releasedKeys = g.releasedKeys[:0]
	g.releasedKeys = inpututil.AppendJustReleasedKeys(g.releasedKeys)

	leftMouseButton := ebiten.MouseButton0
	if inpututil.IsMouseButtonJustReleased(leftMouseButton) {
		x, y := ebiten.CursorPosition()
		g.click(x, y)
	}

	return nil
}

func (g *Game) reset() {
	g.isStart = true
	g.winner = 0
	g.winningLine = []*peg{}
	g.activeButton = nil
	g.pegs = [][]*peg{}
	for i := 0; i < g.columns; i++ {
		g.pegs = append(g.pegs, make([]*peg, g.rows))
	}
	g.button("let's play")
}

func (g *Game) isFinished() bool {
	return g.winner > 0
}

func (g *Game) addPeg(column int) *peg {
	if g.isFinished() {
		fmt.Printf("game is already finished\n")
		return nil
	}
	if column >= len(g.pegs) {
		fmt.Printf("illegal column %v\n", column)
		return nil
	}
	rows := g.pegs[column]
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i] == nil {
			g.playSound("click")
			p := &peg{g: g, player: g.activePlayer, x: column, y: i}
			rows[i] = p
			return p
		}
	}
	fmt.Printf("column %v is already filled", column)
	return nil
}

func (g *Game) playSound(n string) {
	s, ok := g.sounds[n]
	if !ok {
		fmt.Printf("unknown sound name %#v\n", n)
		return
	}
	fmt.Printf("n=%v play()\n", n)
	err := s.Rewind()
	if err != nil {
		fmt.Printf("error rewinding sound n=%v err=%v\n", n, err)
		return
	}
	s.Play() // go p.Play() ?
}

type direction int

const (
	North = iota
	NorthEast
	East
	SouthEast
	South
	SouthWest
	West
	NorthWest
)

var directions = []direction{North, NorthEast, East, SouthEast, South, SouthWest, West, NorthWest}

func (g *Game) nextPos(currentX, currentY int, d direction) (int, int, bool) {
	switch d {
	case North:
		if currentY == 0 {
			return 0, 0, false
		}
		return currentX, currentY - 1, true
	case NorthEast:
		if currentY == 0 {
			return 0, 0, false
		}
		if currentX == g.columns-1 {
			return 0, 0, false
		}
		return currentX + 1, currentY - 1, true
	case East:
		if currentX == g.columns-1 {
			return 0, 0, false
		}
		return currentX + 1, currentY, true
	case SouthEast:
		if currentY == g.rows-1 {
			return 0, 0, false
		}
		if currentX == g.columns-1 {
			return 0, 0, false
		}
		return currentX + 1, currentY + 1, true
	case South:
		if currentY == g.rows-1 {
			return 0, 0, false
		}
		return currentX, currentY + 1, true
	case SouthWest:
		if currentY == g.rows-1 {
			return 0, 0, false
		}
		if currentX == 0 {
			return 0, 0, false
		}
		return currentX - 1, currentY + 1, true
	case West:
		if currentX == 0 {
			return 0, 0, false
		}
		return currentX - 1, currentY, true
	case NorthWest:
		if currentX == 0 {
			return 0, 0, false
		}
		if currentY == 0 {
			return 0, 0, false
		}
		return currentX - 1, currentY - 1, true
	default:
		panic("uhoh")
	}
}

func (g *Game) checkForWinner() (int, bool) {
	for _, c := range g.pegs {
		for _, p := range c {
			if p == nil {
				continue
			}
			winningLine, ok := p.hasFour()
			if ok {
				g.winningLine = winningLine
				fmt.Printf("found winning line, player %v won\n", g.activePlayer)
				printLine(g.winningLine)
				return g.activePlayer, true
			}
		}
	}

	return 0, false
}

func (g *Game) finishTurn() {
	winner, ok := g.checkForWinner()
	if ok {
		g.winner = winner
		g.playSound("cheer")
		g.button("> new game")
		return
	}

	if g.activePlayer == 1 {
		g.activePlayer = 2
	} else {
		g.activePlayer = 1
	}
	fmt.Printf("now it's %v's turn\n", g.activePlayer)
}

func (g *Game) positionToColumn(x, y int) (int, bool) {
	if x > g.screenWidth() || x < 0 {
		return 0, false
	}
	if y > g.screenHeight() || y < 0 {
		return 0, false
	}

	return x / g.blockSize, true
}

func (g *Game) Draw(screen *ebiten.Image) {
	for ci, col := range g.pegs {
		for ri, p := range col {
			op := &ebiten.DrawImageOptions{}
			x := ci * 90
			y := ri * 90
			op.GeoM.Translate(float64(x), float64(y))

			switch {
			case p == nil:
				screen.DrawImage(g.images["empty"], op)
			case len(g.winningLine) > 0 && p.isIn(g.winningLine):
				screen.DrawImage(g.images["pink"], op)
			case p.player == 1:
				screen.DrawImage(g.images["yellow"], op)
			case p.player == 2:
				screen.DrawImage(g.images["red"], op)
			default:
				panic("asihoetnasiohetn")
			}
		}
	}

	if g.activeButton != nil {
		g.drawButton(screen)
	}

	if g.winner > 0 {
		name := "red"
		clr := color.RGBA{0xff, 0, 0, 0xff}
		if g.winner == 1 {
			name = "yellow"
			clr = color.RGBA{0xff, 0xff, 0, 0xff}
		}
		g.message(screen, fmt.Sprintf("%v won!", name), clr)
	}
}

func (g *Game) Layout(outsideWidth, outsideHight int) (int, int) {
	return g.screenWidth(), g.screenHeight()
}
func (g *Game) screenWidth() int  { return g.columns * g.blockSize }
func (g *Game) screenHeight() int { return g.rows * g.blockSize }

func (g *Game) message(screen *ebiten.Image, msg string, clr color.Color) {
	width := len(msg)*14 + 16
	height := 24
	x := g.screenWidth()/2 - width/2
	vector.DrawFilledRect(screen, float32(x), 0, float32(width), float32(height), color.Black, false)
	text.Draw(screen, msg, g.fonts["arcade"], x+10, height/2+7, clr)
	// ebitenutil.DebugPrint(screen, msg)
}

func (g *Game) drawButton(screen *ebiten.Image) {
	x := g.screenWidth()/2 - g.activeButton.width/2
	y := g.screenHeight()/2 - g.activeButton.height/2
	width := g.activeButton.width
	height := g.activeButton.height
	vector.DrawFilledRect(
		screen,
		float32(x),
		float32(y),
		float32(width),
		float32(height),
		color.Black,
		false,
	)
	text.Draw(
		screen,
		g.activeButton.label,
		g.fonts["arcade"],
		x+8,
		y+18,
		color.RGBA{
			0,
			0xff,
			0,
			0xff,
		},
	)
}

func (g *Game) button(label string) {
	width := len(label)*14 + 16
	height := 24
	x := g.screenWidth()/2 - width/2
	y := g.screenHeight()/2 - height/2
	g.activeButton = &button{
		label:  label,
		x:      x,
		y:      y,
		width:  width,
		height: height,
	}
}

func (g *Game) isButtonClick(x, y int) bool {
	if g.activeButton == nil {
		return false
	}

	bx, by, bw, bh := g.activeButton.x, g.activeButton.y, g.activeButton.width, g.activeButton.height
	if x < bx || x > bx+bw {
		return false
	}
	if y < by || y > by+bh {
		return false
	}

	return true
}
