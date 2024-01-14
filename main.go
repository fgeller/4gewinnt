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

//go:embed uhoh.mp3
var uhohMp3 []byte

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
	var maxStreak []*peg
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

		if len(streak) > len(maxStreak) {
			maxStreak = streak
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

	return maxStreak, false
}

type button struct {
	label         string
	x, y          int
	width, height int
}

type message struct {
	label         string
	color         color.Color
	x, y          int
	width, height int
}

type newPeg struct {
	p                *peg
	screenX, screenY int
}

type Game struct {
	pegs            [][]*peg
	pressedTouchIDs []ebiten.TouchID
	releasedKeys    []ebiten.Key

	newPeg *newPeg

	activePlayer int
	winner       int
	winningLine  []*peg
	uhohCount    int

	isStart bool

	columns, rows int
	blockSize     int

	activeButton  *button
	activeMessage *message

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
			Hinting: font.HintingNone,
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

	uhohStream, err := mp3.DecodeWithoutResampling(bytes.NewReader(uhohMp3))
	if err != nil {
		return errors.Wrap(err, "failed to load uhoh.mp3")
	}
	uhoh, err := audioContext.NewPlayer(uhohStream)
	if err != nil {
		return errors.Wrap(err, "failed to create uhoh player")
	}

	g.sounds["click"] = click
	g.sounds["cheer"] = cheer
	g.sounds["uhoh"] = uhoh

	return nil
}

func (g *Game) click(x, y int) {
	if g.isStart {
		g.activeButton = nil
		g.activeMessage = nil
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
	p := g.addPeg(col)
	if p != nil {
		g.newPeg = &newPeg{p: p, screenX: col * g.blockSize, screenY: 0}
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

	// 60 tps
	if g.newPeg != nil {
		maxY := g.newPeg.p.y * g.blockSize
		if g.newPeg.screenY == maxY {
			g.newPeg = nil
			g.playSound("click")
		} else {
			next := g.newPeg.screenY + 15
			g.newPeg.screenY = min(next, maxY)
		}
	}

	return nil
}

func (g *Game) reset() {
	g.isStart = true
	g.winner = 0
	g.winningLine = []*peg{}
	g.uhohCount = 0
	g.activeButton = nil
	g.activeMessage = nil
	g.pegs = [][]*peg{}
	g.newPeg = nil
	for i := 0; i < g.columns; i++ {
		g.pegs = append(g.pegs, make([]*peg, g.rows))
	}
	g.message("let's play", color.RGBA{0xff, 0xff, 0xff, 0xff})
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
	err := s.Rewind()
	if err != nil {
		fmt.Printf("error rewinding sound n=%v err=%v\n", n, err)
		return
	}
	s.Play()
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
	pegCount := 0
	uhohCount := 0
	for _, c := range g.pegs {
		for _, p := range c {
			if p == nil {
				continue
			}
			pegCount++
			streak, ok := p.hasFour()
			if ok {
				g.winningLine = streak
				fmt.Printf("found winning line, player %v won\n", g.activePlayer)
				printLine(g.winningLine)
				return g.activePlayer, true
			}

			if len(streak) == 3 {
				uhohCount += 1
			}
		}
	}

	if pegCount == g.columns*g.rows {
		fmt.Println("draw!")
		return 3, true
	}

	if uhohCount > g.uhohCount {
		g.uhohCount = uhohCount
		g.playSound("uhoh")
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
	vector.DrawFilledRect(screen, 0, 0, float32(g.screenWidth()), float32(g.screenHeight()), color.White, false)

	// animated peg

	if g.newPeg != nil {
		p := g.newPeg.p
		op := &ebiten.DrawImageOptions{}
		x := g.newPeg.screenX
		y := g.newPeg.screenY
		op.GeoM.Translate(float64(x), float64(y))
		switch {
		case p.player == 1:
			screen.DrawImage(g.images["yellow"], op)
		case p.player == 2:
			screen.DrawImage(g.images["red"], op)
		default:
			panic("asihoetnasiohetn")
		}
	}

	// grid

	for ci, col := range g.pegs {
		for ri := range col {
			op := &ebiten.DrawImageOptions{}
			x := ci * g.blockSize
			y := ri * g.blockSize
			op.GeoM.Translate(float64(x), float64(y))
			screen.DrawImage(g.images["empty"], op)
		}
	}

	// set pegs
	for ci, col := range g.pegs {
		for ri, p := range col {
			if p == nil {
				continue
			}
			if g.newPeg != nil && g.newPeg.p.x == p.x && g.newPeg.p.y == p.y {
				continue
			}

			op := &ebiten.DrawImageOptions{}
			x := ci * g.blockSize
			y := ri * g.blockSize
			op.GeoM.Translate(float64(x), float64(y))

			switch {
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

	g.drawButton(screen)
	g.drawMessage(screen)

	if g.winner > 0 {
		var msg string
		clr := color.RGBA{0xff, 0, 0xff, 0xff}
		switch g.winner {
		case 1:
			msg = "yellow won!"
			clr = color.RGBA{0xff, 0xff, 0, 0xff}
		case 2:
			msg = "red won!"
			clr = color.RGBA{0xff, 0, 0, 0xff}
		default:
			msg = "it's a draw!"
		}
		g.message(msg, clr)
	}
}

func (g *Game) Layout(outsideWidth, outsideHight int) (int, int) {
	return g.screenWidth(), g.screenHeight()
}
func (g *Game) screenWidth() int  { return g.columns * g.blockSize }
func (g *Game) screenHeight() int { return g.rows * g.blockSize }

func (g *Game) message(msg string, clr color.Color) {
	width := len(msg)*14 + 16
	height := 24
	x := g.screenWidth()/2 - width/2
	y := g.screenHeight()/2 - height/2 - height // offset from button
	g.activeMessage = &message{
		label:  msg,
		color:  clr,
		x:      x,
		y:      y,
		width:  width,
		height: height,
	}
}

func (g *Game) drawMessage(screen *ebiten.Image) {
	if g.activeMessage == nil {
		return
	}
	msg := g.activeMessage
	width := msg.width
	if g.activeButton != nil {
		width = max(width, g.activeButton.width)
	}
	x := g.screenWidth()/2 - width/2
	vector.DrawFilledRect(
		screen,
		float32(x),
		float32(msg.y),
		float32(width),
		float32(msg.height),
		color.Black,
		false,
	)
	text.Draw(
		screen,
		msg.label,
		g.fonts["arcade"],
		msg.x+8,
		msg.y+18,
		msg.color,
	)
}

func (g *Game) drawButton(screen *ebiten.Image) {
	if g.activeButton == nil {
		return
	}

	b := g.activeButton
	width := b.width
	if g.activeMessage != nil {
		width = max(width, g.activeMessage.width)
	}
	x := g.screenWidth()/2 - width/2
	vector.DrawFilledRect(
		screen,
		float32(x),
		float32(b.y),
		float32(width),
		float32(b.height),
		color.Black,
		false,
	)
	text.Draw(
		screen,
		b.label,
		g.fonts["arcade"],
		b.x+8,
		b.y+18,
		color.RGBA{0, 0xff, 0, 0xff},
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
