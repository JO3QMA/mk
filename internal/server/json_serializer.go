package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// fastJSONSerializer was previously backed by goccy/go-json (#507) but
// goccy v0.10.6 panics with `ptrToString` nil-deref on certain hot path
// payloads (timeline notes containing remote users). 一旦 stdlib
// `encoding/json` に戻して安定運用、performance 改善は別 issue で
// goccy 上げ or 別 encoder 検討。
type fastJSONSerializer struct{}

// Serialize writes i as JSON to the response. indent が空でなければ
// pretty-print する (echo 標準と同じセマンティクス)。
func (fastJSONSerializer) Serialize(c echo.Context, i interface{}, indent string) error {
	enc := json.NewEncoder(c.Response())
	if indent != "" {
		enc.SetIndent("", indent)
	}
	return enc.Encode(i)
}

// Deserialize parses request body JSON into i. 不正な JSON は echo の
// HTTPError(400) に翻訳し、そのまま返すと echo の error handler に渡る。
func (fastJSONSerializer) Deserialize(c echo.Context, i interface{}) error {
	if err := json.NewDecoder(c.Request().Body).Decode(i); err != nil {
		var ute *json.UnmarshalTypeError
		if errors.As(err, &ute) {
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("Unmarshal type error: expected=%v, got=%v, field=%v, offset=%v",
					ute.Type, ute.Value, ute.Field, ute.Offset)).SetInternal(err)
		}
		var se *json.SyntaxError
		if errors.As(err, &se) {
			return echo.NewHTTPError(http.StatusBadRequest,
				fmt.Sprintf("Syntax error: offset=%v, error=%v", se.Offset, se.Error())).SetInternal(err)
		}
		return err
	}
	return nil
}
