package reversi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFederationIDCache_NilRedis(t *testing.T) {
	c := NewFederationIDCache(nil)
	ctx := context.Background()
	c.Set(ctx, "sess1", "game1") // no panic
	_, err := c.Get(ctx, "sess1")
	assert.Error(t, err)
}

func TestIsValidUpdateKey(t *testing.T) {
	assert.True(t, IsValidUpdateKey("map"))
	assert.True(t, IsValidUpdateKey("isLlotheo"))
	assert.True(t, IsValidUpdateKey("timeLimitForEachTurn"))
	assert.False(t, IsValidUpdateKey("invalid"))
	assert.False(t, IsValidUpdateKey(""))
}

func TestWinner_WhiteWins(t *testing.T) {
	g := NewGame([]string{"ww", "w-"}, Options{})
	g.Turn = nil
	w := g.Winner()
	assert.NotNil(t, w)
	assert.Equal(t, White, *w)
}

func TestNewGame_WStone(t *testing.T) {
	g := NewGame([]string{"w-", "-b"}, Options{})
	assert.Equal(t, 1, g.WhiteCount())
	assert.Equal(t, 1, g.BlackCount())
}

func TestCalcTurn_SamePrevColor(t *testing.T) {
	g := NewGame([]string{"bw", "--"}, Options{})
	// bが(0,1)に置くと、wが置けない場合bの連続ターン
	if g.CanPut(Black, 2) {
		g.PutStone(2)
	}
}

func TestEffectsInLine_LoopedReturn(t *testing.T) {
	// ループボードで自分の位置に戻るケース
	g := NewGame([]string{"bw"}, Options{LoopedBoard: true})
	effects := g.effectsInLine(Black, White, 0, 1, 0)
	// 1マスしかないので戻ってくる
	_ = effects
}

func TestEffectsInLine_NullCell(t *testing.T) {
	// null cell でeffectsが打ち切られるケース
	g := NewGame([]string{"b.w"}, Options{})
	effects := g.effectsInLine(Black, White, 0, 1, 0)
	assert.Empty(t, effects) // null cellで打ち切り
}
