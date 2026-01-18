package loggo

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/f0reth/golog/internal/buffer"
)

// ANSIカラーコード
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

// Handler は指定されたフォーマットでログを出力するハンドラー
type Handler struct {
	out               io.Writer
	minLevel          slog.Level
	timeFormat        string
	attrs             []slog.Attr
	groups            []string
	useColors         bool                                        // 色を使用するかどうかのフラグ
	addSource         bool                                        // ソースファイルと行番号を追加するかどうか
	replaceAttr       func(groups []string, a slog.Attr) slog.Attr // 属性を変換するコールバック
	mu                *sync.Mutex                                 // スレッドセーフな書き込みのためのミューテックス
	preformattedAttrs []byte                                      // 事前フォーマット済みの属性（パフォーマンス最適化）
}

// Options はカスタムハンドラーのオプション
type Options struct {
	Level       slog.Leveler
	UseColors   bool
	TimeFormat  string                                      // 時刻フォーマット（空の場合は "2006-01-02 15:04:05.000" を使用）
	AddSource   bool                                        // ソースファイルと行番号を追加するかどうか
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr // 属性を変換するコールバック
}

// NewHandler は新しいカスタムハンドラーを作成します
func NewHandler(w io.Writer, opts *Options) *Handler {
	var level slog.Level
	useColors := false
	addSource := false
	var replaceAttr func(groups []string, a slog.Attr) slog.Attr
	timeFormat := "2006-01-02 15:04:05.000" // デフォルト: ミリ秒までのフォーマット

	if opts != nil {
		if opts.Level != nil {
			level = opts.Level.Level()
		}
		useColors = opts.UseColors
		addSource = opts.AddSource
		replaceAttr = opts.ReplaceAttr
		if opts.TimeFormat != "" {
			timeFormat = opts.TimeFormat
		}
	}

	return &Handler{
		out:         w,
		minLevel:    level,
		timeFormat:  timeFormat,
		attrs:       []slog.Attr{},
		groups:      []string{},
		useColors:   useColors,
		addSource:   addSource,
		replaceAttr: replaceAttr,
		mu:          &sync.Mutex{},
	}
}

// Enabled はログレベルが有効かどうかを判断します
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

// Handle はログレコードを処理します
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if !h.Enabled(ctx, r.Level) {
		return nil
	}

	// Buffer Pool からバッファを取得
	buf := buffer.New()
	defer buf.Free()

	// 時刻属性の処理（ReplaceAttrが設定されている場合は適用）
	timeAttr := slog.Time(slog.TimeKey, r.Time)
	if h.replaceAttr != nil {
		timeAttr = h.replaceAttr(nil, timeAttr)
	}
	// 時刻が無視されていない場合は出力
	if timeAttr.Key != "" {
		buf.WriteByte('[')
		*buf = r.Time.AppendFormat(*buf, h.timeFormat)
		buf.WriteString("] ")
	}

	// レベル属性の処理（ReplaceAttrが設定されている場合は適用）
	levelAttr := slog.Any(slog.LevelKey, r.Level)
	if h.replaceAttr != nil {
		levelAttr = h.replaceAttr(nil, levelAttr)
	}
	// レベルが無視されていない場合は出力
	if levelAttr.Key != "" {
		levelStr := h.formatLevelWithColor(r.Level)
		buf.WriteByte('[')
		buf.WriteString(levelStr)
		buf.WriteString("] ")
	}

	// メッセージ属性の処理（ReplaceAttrが設定されている場合は適用）
	msgAttr := slog.String(slog.MessageKey, r.Message)
	if h.replaceAttr != nil {
		msgAttr = h.replaceAttr(nil, msgAttr)
	}
	// メッセージが無視されていない場合は出力
	if msgAttr.Key != "" {
		buf.WriteString("msg=")
		formattedMsg, msgErr := formatValue(msgAttr.Value.Any())
		if msgErr != nil {
			buf.WriteString("\"!ERROR:")
			buf.WriteString(msgErr.Error())
			buf.WriteByte('"')
		} else {
			buf.WriteString(formattedMsg)
		}
	}

	// 事前フォーマット済みの属性を追加
	if len(h.preformattedAttrs) > 0 {
		buf.Write(h.preformattedAttrs)
	}

	// ソース情報を追加
	if h.addSource {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			// ファイル名のみを取得（フルパスではなく）
			file := filepath.Base(f.File)
			// "file.go:42" 形式でフォーマット
			sourceStr := file + ":" + strconv.Itoa(f.Line)

			// ソース属性の処理（ReplaceAttrが設定されている場合は適用）
			sourceAttr := slog.String(slog.SourceKey, sourceStr)
			if h.replaceAttr != nil {
				sourceAttr = h.replaceAttr(nil, sourceAttr)
			}
			// ソースが無視されていない場合は出力
			if sourceAttr.Key != "" {
				buf.WriteString(" ")
				// キーをエスケープ（必要な場合）
				if needsQuoting(sourceAttr.Key) {
					buf.WriteString(strconv.Quote(sourceAttr.Key))
				} else {
					buf.WriteString(sourceAttr.Key)
				}
				buf.WriteString("=")
				formattedSource, _ := formatValue(sourceAttr.Value.Any())
				buf.WriteString(formattedSource)
			}
		}
	}

	// レコードの属性を追加
	r.Attrs(func(attr slog.Attr) bool {
		appendAttr(buf, attr.Key, attr.Value, h.groups, h.replaceAttr) // レコードの属性は現在のグループで囲む
		return true
	})

	buf.WriteByte('\n')

	// スレッドセーフな書き込みのためにロックを取得
	h.mu.Lock()
	_, err := h.out.Write(*buf)
	h.mu.Unlock()
	return err
}

// needsQuoting はキーにクォートが必要かどうかを判定します
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		// スペース、制御文字、=、"、DEL文字のいずれかが含まれる場合はクォートが必要
		if r <= ' ' || r == '=' || r == '"' || r == 0x7f {
			return true
		}
	}
	return false
}

func appendAttr(buf *buffer.Buffer, key string, value slog.Value, groups []string, replaceAttr func(groups []string, a slog.Attr) slog.Attr) {
	// ReplaceAttr コールバックが設定されている場合は適用
	attr := slog.Attr{Key: key, Value: value}
	if replaceAttr != nil {
		attr = replaceAttr(groups, attr)
		// 空のキーが返された場合は属性を無視
		if attr.Key == "" {
			return
		}
	}

	buf.WriteByte(' ') // キーの前にスペース

	// グループプレフィックスを付ける
	if len(groups) > 0 {
		for _, group := range groups {
			// グループ名もエスケープが必要な場合はクォート
			if needsQuoting(group) {
				buf.WriteString(strconv.Quote(group))
			} else {
				buf.WriteString(group)
			}
			buf.WriteByte('.')
		}
	}

	// キーをエスケープ（必要な場合）
	if needsQuoting(attr.Key) {
		buf.WriteString(strconv.Quote(attr.Key))
	} else {
		buf.WriteString(attr.Key)
	}
	buf.WriteByte('=')
	// formatValueは slog.Value.Any() を受け取るので、value.Any() を渡す
	jsonStr, err := formatValue(attr.Value.Any())
	if err != nil {
		// slog.TextHandlerに倣ったエラー表示（fmt.Fprintf を避ける）
		buf.WriteString("\"!ERROR:")
		buf.WriteString(err.Error())
		buf.WriteByte('"')
		return
	}
	buf.WriteString(jsonStr)
}

// formatLevelWithColor はログレベルを色付きでフォーマットします
func (h *Handler) formatLevelWithColor(level slog.Level) string {
	levelStr := formatLevel(level)

	if !h.useColors {
		return levelStr
	}

	// レベルに応じた色を適用（fmt.Sprintf を避けて直接結合）
	var colorCode string
	switch level {
	case slog.LevelDebug:
		colorCode = colorCyan
	case slog.LevelInfo:
		colorCode = colorGreen
	case slog.LevelWarn:
		colorCode = colorYellow
	case slog.LevelError:
		colorCode = colorRed
	default:
		colorCode = colorWhite
	}

	return colorCode + levelStr + colorReset
}

// formatValue は値を適切な形式に変換します
func formatValue(v any) (string, error) {
	// nullの場合
	if v == nil {
		return "null", nil
	}

	// slog.LogValuer インターフェースのチェック（標準ライブラリのサポート）
	if lv, ok := v.(slog.LogValuer); ok {
		// LogValue() を呼び出して slog.Value を取得
		value := lv.LogValue()
		// slog.Value.Any() で実際の値を取得して再帰的に処理
		return formatValue(value.Any())
	}

	// 文字列の場合は strconv.Quote を使用して安全にエスケープ
	// すべてのASCII制御文字や特殊文字を適切に処理
	if s, ok := v.(string); ok {
		return strconv.Quote(s), nil
	}

	// 数値、真偽値の場合は strconv を使用（fmt.Sprintf より高速）
	switch v := v.(type) {
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case LogFormatter:
		// LogFormatterインターフェースを実装している場合は、そのメソッドを呼び出す
		return v.FormatForLog()
	}

	// 構造体処理
	rv := reflect.ValueOf(v)

	// ポインタの場合はnilチェックして実体を取得
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "null", nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Struct {
		// 通常の構造体をJSONに変換
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	// それ以外の型はJSONとしてマーシャル
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LogFormatter はログ出力のためのカスタムフォーマットを提供するインターフェース
type LogFormatter interface {
	FormatForLog() (string, error)
}

// WithAttrs は新しい属性を持つハンドラーを返します
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	newHandler := *h // 既存のハンドラの値をコピー

	// groupsをコピー
	newHandler.groups = make([]string, len(h.groups))
	copy(newHandler.groups, h.groups)

	// 属性を事前にフォーマット（パフォーマンス最適化）
	buf := buffer.New()
	defer buf.Free()

	// 既存の事前フォーマット済み属性をコピー
	if len(h.preformattedAttrs) > 0 {
		buf.Write(h.preformattedAttrs)
	}

	// 新しい属性をフォーマットして追加
	for _, attr := range attrs {
		appendAttr(buf, attr.Key, attr.Value, h.groups, h.replaceAttr)
	}

	// 事前フォーマット済み属性として保存
	newHandler.preformattedAttrs = make([]byte, buf.Len())
	copy(newHandler.preformattedAttrs, *buf)

	// attrsフィールドは互換性のために保持（現在は使用されていない）
	newHandler.attrs = nil

	// ミューテックスは共有する（標準ライブラリと同じ戦略）
	// newHandler.mu = h.mu (構造体のコピーで既に共有されている)
	return &newHandler
}

// WithGroup は新しいグループを持つハンドラーを返します
func (h *Handler) WithGroup(name string) slog.Handler {
	newHandler := *h // 既存のハンドラの値をコピー

	// preformattedAttrs をコピー
	if len(h.preformattedAttrs) > 0 {
		newHandler.preformattedAttrs = make([]byte, len(h.preformattedAttrs))
		copy(newHandler.preformattedAttrs, h.preformattedAttrs)
	}

	// groupsをコピーして追加
	newHandler.groups = make([]string, len(h.groups)+1)
	copy(newHandler.groups, h.groups)
	newHandler.groups[len(h.groups)] = name

	// ミューテックスは共有する（標準ライブラリと同じ戦略）
	// newHandler.mu = h.mu (構造体のコピーで既に共有されている)
	return &newHandler
}

// formatLevel はログレベルを指定された形式にフォーマットします
func formatLevel(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return " INFO" // スペースを含めて幅を統一
	case slog.LevelWarn:
		return " WARN" // スペースを含めて幅を統一
	case slog.LevelError:
		return "ERROR"
	default:
		// fmt.Sprintf を避けて strings パッケージで5文字幅に揃える
		s := level.String()
		if len(s) < 5 {
			return strings.Repeat(" ", 5-len(s)) + s
		}
		return s
	}
}
