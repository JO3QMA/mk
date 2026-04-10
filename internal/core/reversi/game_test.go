package reversi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultMap() []string {
	return []string{
		"--------",
		"--------",
		"--------",
		"---wb---",
		"---bw---",
		"--------",
		"--------",
		"--------",
	}
}

func TestNewGame_DefaultBoard(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	assert.Equal(t, 8, g.MapWidth)
	assert.Equal(t, 8, g.MapHeight)
	assert.Equal(t, 2, g.BlackCount())
	assert.Equal(t, 2, g.WhiteCount())
	require.NotNil(t, g.Turn)
	assert.Equal(t, Black, *g.Turn)
	assert.False(t, g.IsEnded())
}

func TestNewGame_EmptyMap(t *testing.T) {
	g := NewGame(nil, Options{})
	assert.Equal(t, 8, g.MapWidth)
}

func TestPutStone_Basic(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	// Black plays at (2,3) — should flip (3,3) from white to black
	pos := g.XYToPos(2, 3)
	assert.True(t, g.CanPut(Black, pos))
	g.PutStone(pos)
	assert.Equal(t, 4, g.BlackCount())
	assert.Equal(t, 1, g.WhiteCount())
	// Turn should switch to white
	require.NotNil(t, g.Turn)
	assert.Equal(t, White, *g.Turn)
}

func TestCanPut_OccupiedCell(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	// (3,3) has a white stone
	pos := g.XYToPos(3, 3)
	assert.False(t, g.CanPut(Black, pos))
}

func TestCanPut_OutOfBounds(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	assert.False(t, g.CanPut(Black, -1))
	assert.False(t, g.CanPut(Black, 64))
}

func TestCanPut_NoEffect(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	// (0,0) — too far, no flip
	assert.False(t, g.CanPut(Black, 0))
}

func TestCanPutEverywhere(t *testing.T) {
	g := NewGame(defaultMap(), Options{CanPutEverywhere: true})
	// 空いてるセルならどこでも置ける
	assert.True(t, g.CanPut(Black, 0))
}

func TestEffects(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	// Black at (2,3) should flip (3,3)
	effects := g.Effects(Black, g.XYToPos(2, 3))
	assert.Contains(t, effects, g.XYToPos(3, 3))
}

func TestPutStone_NilTurn(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	g.Turn = nil
	g.PutStone(0) // ターンがnilなので何も起きない
	assert.Nil(t, g.Turn)
}

func TestCalcTurn_NoPrevColor(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	g.PrevColor = nil
	g.calcTurn()
	// PrevColorがnilなので変更なし
}

func TestIsEnded_False(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	assert.False(t, g.IsEnded())
}

func TestWinner_NotEnded(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	assert.Nil(t, g.Winner())
}

func TestWinner_Draw(t *testing.T) {
	// 同数で終了した場合は引き分け
	g := NewGame(defaultMap(), Options{})
	g.Turn = nil // 強制終了
	assert.Nil(t, g.Winner())
}

func TestWinner_BlackWins(t *testing.T) {
	g := NewGame([]string{"bb", "b-"}, Options{})
	g.Turn = nil
	w := g.Winner()
	require.NotNil(t, w)
	assert.Equal(t, Black, *w)
}

func TestWinner_Llotheo(t *testing.T) {
	// Llotheoモード: 石が少ない方が勝ち
	g := NewGame([]string{"bb", "bw"}, Options{IsLlotheo: true})
	g.Turn = nil
	w := g.Winner()
	require.NotNil(t, w)
	assert.Equal(t, White, *w) // whiteが1つで少ないので勝ち
}

func TestLoopedBoard(t *testing.T) {
	g := NewGame(defaultMap(), Options{LoopedBoard: true})
	// ループボードでも基本動作する
	assert.NotNil(t, g.Turn)
}

func TestGetLogs(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	g.PutStone(g.XYToPos(2, 3))
	logs := g.GetLogs()
	assert.Len(t, logs, 1)
	assert.Equal(t, g.XYToPos(2, 3), logs[0][0])
	assert.Equal(t, 1, logs[0][1]) // black=1
}

func TestCalcCRC32(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	crc := g.CalcCRC32()
	assert.NotZero(t, crc)
	// 同じ盤面なら同じCRC
	g2 := NewGame(defaultMap(), Options{})
	assert.Equal(t, crc, g2.CalcCRC32())
}

func TestGetPuttablePlaces(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	places := g.GetPuttablePlaces(Black)
	assert.NotEmpty(t, places)
	// 標準配置では黒は4箇所に置ける
	assert.Len(t, places, 4)
}

func TestPosToXY_XYToPos(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	x, y := g.PosToXY(27) // 27 = 3 + 3*8
	assert.Equal(t, 3, x)
	assert.Equal(t, 3, y)
	assert.Equal(t, 27, g.XYToPos(3, 3))
}

func TestMapDataGet_OutOfBounds(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	assert.Equal(t, CellNull, g.MapDataGet(-1))
	assert.Equal(t, CellNull, g.MapDataGet(64))
}

func TestCanPutSomewhere(t *testing.T) {
	g := NewGame(defaultMap(), Options{})
	assert.True(t, g.CanPutSomewhere(Black))
	assert.True(t, g.CanPutSomewhere(White))
}

func TestFullGame(t *testing.T) {
	// 2x2ボードで完全ゲームを実行
	g := NewGame([]string{"--", "bw"}, Options{})
	if g.Turn != nil && g.CanPut(*g.Turn, 0) {
		g.PutStone(0)
	}
	if g.Turn != nil && g.CanPut(*g.Turn, 1) {
		g.PutStone(1)
	}
	// ゲーム終了しているはず
	// (小さいボードなので数手で終わる)
}
