package server

import (
	"errors"
	"fmt"
	"net/http"

	gojson "github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
)

// fastJSONSerializer is the echo.JSONSerializer implementation backed by
// goccy/go-json. echo の `c.JSON()` / `c.Bind()` 経路を全て差し替えるため、
// ハンドラ側のコードはそのまま動かしつつ encoding/json の reflection cost を
// 減らせる (#507 / #413 #2)。
//
// stdlib の DefaultJSONSerializer と同じく、Decode 時の TypeError /
// SyntaxError を 400 BadRequest に翻訳して echo の error pipeline に流す。
type fastJSONSerializer struct{}

// Serialize writes i as JSON to the response. indent が空でなければ
// pretty-print する (echo 標準と同じセマンティクス)。
func (fastJSONSerializer) Serialize(c echo.Context, i interface{}, indent string) error {
	enc := gojson.NewEncoder(c.Response())
	if indent != "" {
		enc.SetIndent("", indent)
	}
	return enc.Encode(i)
}

// Deserialize parses request body JSON into i. 不正な JSON は echo の
// HTTPError(400) に翻訳し、そのまま返すと echo の error handler に渡る。
func (fastJSONSerializer) Deserialize(c echo.Context, i interface{}) error {
	if err := gojson.NewDecoder(c.Request().Body).Decode(i); err != nil {
		// goccy/go-json は stdlib 互換の errorタイプを返すので、
		// errors.As で TypeError / SyntaxError を取り出して echo HTTPError
		// に整形する。stdlib DefaultJSONSerializer と同じ挙動。
		var ute *gojson.UnmarshalTypeError
		if errors.As(err, &ute) {
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("Unmarshal type error: expected=%v, got=%v, field=%v, offset=%v",
					ute.Type, ute.Value, ute.Field, ute.Offset)).SetInternal(err)
		}
		var se *gojson.SyntaxError
		if errors.As(err, &se) {
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("Syntax error: offset=%v, error=%v", se.Offset, se.Error())).SetInternal(err)
		}
		return err
	}
	return nil
}
