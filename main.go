package main

import (
	"errors"
	"fmt"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func main() {
	g := newGame(7, 6)

	ebiten.SetWindowSize(g.screenWidth(), g.screenHeight())
	ebiten.SetWindowTitle("4 Gewinnt")

	err := ebiten.RunGame(g)
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

func (p *peg) neighbor(direction int) *peg {
	nextX, nextY, onBoard := p.g.nextPos(p.x, p.y, direction)
	if !onBoard {
		return nil
	}
	return p.g.pegs[nextX][nextY]
}

func (p *peg) hasFour() ([]*peg, bool) {
	checkLine := func(directionA, directionB int) ([]*peg, bool) {
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

type Game struct {
	pegs            [][]*peg
	pressedTouchIDs []ebiten.TouchID
	releasedKeys    []ebiten.Key

	activePlayer int
	winner       int
	winningLine  []*peg

	columns, rows int
	blockSize     int
}

func newGame(columns, rows int) *Game {
	g := &Game{
		blockSize:    90,
		activePlayer: 1,
		pegs:         [][]*peg{},
	}

	g.columns = columns
	g.rows = rows

	g.pressedTouchIDs = make([]ebiten.TouchID, 0, 48)
	g.releasedKeys = make([]ebiten.Key, 0, 48)
	g.reset()

	return g
}

func (g *Game) Update() error {
	leftMouseButton := ebiten.MouseButton0
	rightMouseButton := ebiten.MouseButton2

	g.pressedTouchIDs = g.pressedTouchIDs[:0]
	g.pressedTouchIDs = inpututil.AppendJustPressedTouchIDs(g.pressedTouchIDs)

	for _, tid := range g.pressedTouchIDs {
		if !inpututil.IsTouchJustReleased(tid) {
			continue
		}
		touchX, touchY := inpututil.TouchPositionInPreviousTick(tid)
		fmt.Printf("touchX=%v touchY=%v \n", touchX, touchY)
	}

	g.releasedKeys = g.releasedKeys[:0]
	g.releasedKeys = inpututil.AppendJustReleasedKeys(g.releasedKeys)

	for _, key := range g.releasedKeys {
		fmt.Printf("key=%#v \n", key)
	}

	if inpututil.IsMouseButtonJustReleased(leftMouseButton) {
		x, y := ebiten.CursorPosition()
		col, err := g.positionToColumn(x, y)
		if err == errOutOfBounds {
			col = -1
		}
		fmt.Printf("left click x=%v y=%v col=%v\n", x, y, col)
		p := g.addPeg(col)
		if p != nil {
			g.finishTurn()
		}
	}

	if inpututil.IsMouseButtonJustReleased(rightMouseButton) {
		fmt.Printf("right click\n")
		if g.winner > 0 {
			fmt.Printf("new game!\n")
			g.reset()
		}
	}

	return nil
}

func (g *Game) reset() {
	g.winner = 0
	g.winningLine = []*peg{}
	g.pegs = [][]*peg{}
	for i := 0; i < g.columns; i++ {
		g.pegs = append(g.pegs, make([]*peg, g.rows))
	}
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

var directions = []int{North, NorthEast, East, SouthEast, South, SouthWest, West, NorthWest}

func (g *Game) nextPos(currentX, currentY int, direction int) (int, int, bool) {
	switch direction {
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
				fmt.Printf("found winning line\n")
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
		return
	}

	if g.activePlayer == 1 {
		g.activePlayer = 2
	} else {
		g.activePlayer = 1
	}
	fmt.Printf("now it's %v's turn\n", g.activePlayer)
}

var errOutOfBounds = errors.New("out of bounds")

func (g *Game) positionToColumn(x, y int) (int, error) {
	if x > g.screenWidth() || x < 0 {
		return 0, errOutOfBounds
	}
	if y > g.screenHeight() || y < 0 {
		return 0, errOutOfBounds
	}

	return x / g.blockSize, nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	inactivePeg := ebiten.NewImage(g.blockSize, g.blockSize)
	inactivePeg.Fill(color.RGBA{0, 0, 0xff, 0xff})
	player1Peg := ebiten.NewImage(g.blockSize, g.blockSize)
	player1Peg.Fill(color.RGBA{0xff, 0, 0, 0xff})
	player2Peg := ebiten.NewImage(g.blockSize, g.blockSize)
	player2Peg.Fill(color.RGBA{0xff, 0xff, 0, 0xff})
	winningPeg := ebiten.NewImage(g.blockSize, g.blockSize)
	winningPeg.Fill(color.RGBA{0xff, 0xc0, 0xcb, 0xff})

	for ci, col := range g.pegs {
		for ri, p := range col {
			op := &ebiten.DrawImageOptions{}
			x := ci * 90
			y := ri * 90
			op.GeoM.Translate(float64(x), float64(y))

			switch {
			case p == nil:
				screen.DrawImage(inactivePeg, op)
			case len(g.winningLine) > 0 && p.isIn(g.winningLine):
				screen.DrawImage(winningPeg, op)
			case p.player == 1:
				screen.DrawImage(player1Peg, op)
			case p.player == 2:
				screen.DrawImage(player2Peg, op)
			default:
				panic("asihoetnasiohetn")
			}
		}
	}

	if g.winner > 0 {
		ebitenutil.DebugPrint(screen, fmt.Sprintf("player %v won!", g.winner))
	}
}

func (g *Game) Layout(outsideWidth, outsideHight int) (int, int) {
	return g.screenWidth(), g.screenHeight()
}
func (g *Game) screenWidth() int  { return g.columns * g.blockSize }
func (g *Game) screenHeight() int { return g.rows * g.blockSize }
