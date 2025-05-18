package loggo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
)

// ANSIカラーコード
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

// Handler は指定されたフォーマットでログを出力するハンドラー
type Handler struct {
	out        io.Writer
	minLevel   slog.Level
	timeFormat string
	attrs      []slog.Attr
	groups     []string
	useColors  bool // 色を使用するかどうかのフラグ
}

// Options はカスタムハンドラーのオプション
type Options struct {
	Level     slog.Leveler
	UseColors bool
}

// NewHandler は新しいカスタムハンドラーを作成します
func NewHandler(w io.Writer, opts *Options) *Handler {
	var level slog.Level
	useColors := false

	if opts != nil {
		if opts.Level != nil {
			level = opts.Level.Level()
		}
		useColors = opts.UseColors
	}

	return &Handler{
		out:        w,
		minLevel:   level,
		timeFormat: "2006-01-02 15:04:05.000", // ミリ秒までのフォーマット
		attrs:      []slog.Attr{},
		groups:     []string{},
		useColors:  useColors,
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

	// 時刻のフォーマット
	timeStr := r.Time.Format(h.timeFormat)

	// レベルのフォーマット（色付き）
	levelStr := h.formatLevelWithColor(r.Level)

	var sb strings.Builder
	fmt.Fprintf(&sb, "[%s] [%s] msg=", timeStr, levelStr)
	formattedMsg, _ := formatValue(r.Message) // エラーハンドリングは別途検討
	sb.WriteString(formattedMsg)

	// 属性の追加
	for _, attr := range h.attrs {
		appendAttr(&sb, attr.Key, attr.Value)
	}
	r.Attrs(func(attr slog.Attr) bool {
		appendAttr(&sb, attr.Key, attr.Value)
		return true
	})

	sb.WriteString("\n")
	_, err := fmt.Fprint(h.out, sb.String())
	return err
}

func appendAttr(sb *strings.Builder, key string, value slog.Value) {
	sb.WriteString(" ") // キーの前にスペース
	sb.WriteString(key)
	sb.WriteString("=")
	// formatValueは slog.Value.Any() を受け取るので、value.Any() を渡す
	jsonStr, err := formatValue(value.Any())
	if err != nil {
		// slog.TextHandlerに倣ったエラー表示
		fmt.Fprintf(sb, "\"!ERROR:%v\"", err)
		return
	}
	sb.WriteString(jsonStr)
}

// formatLevelWithColor はログレベルを色付きでフォーマットします
func (h *Handler) formatLevelWithColor(level slog.Level) string {
	levelStr := formatLevel(level)

	if !h.useColors {
		return levelStr
	}

	// レベルに応じた色を適用
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

	return fmt.Sprintf("%s%s%s", colorCode, levelStr, colorReset)
}

// formatValue は値を適切な形式に変換します
func formatValue(v any) (string, error) {
	// nullの場合
	if v == nil {
		return "null", nil
	}

	// 文字列の場合は引用符で囲む
	if s, ok := v.(string); ok {
		return fmt.Sprintf("\"%s\"", escapeString(s)), nil
	}

	// 数値、真偽値の場合はそのまま文字列化
	switch v := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return fmt.Sprintf("%v", v), nil
	case LogFormatter:
		// LogFormatterインターフェースを実装している場合は、そのメソッドを呼び出す
		return v.FormatForLog()
	}

	// 構造体処理
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Struct || (rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Struct) {
		// LogFormatterインターフェースを実装しているか確認
		if formatter, ok := v.(LogFormatter); ok {
			return formatter.FormatForLog()
		}

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

// escapeString は文字列内の特殊文字をエスケープします
func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// WithAttrs は新しい属性を持つハンドラーを返します
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := *h // 既存のハンドラの値をコピー
	newHandler.attrs = make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newHandler.attrs, h.attrs)
	copy(newHandler.attrs[len(h.attrs):], attrs)
	// groupsも同様にコピーする必要がある (元のハンドラのgroupsを変更しないため)
	newHandler.groups = make([]string, len(h.groups))
	copy(newHandler.groups, h.groups)
	return &newHandler
}

// WithGroup は新しいグループを持つハンドラーを返します
func (h *Handler) WithGroup(name string) slog.Handler {
	newHandler := *h // 既存のハンドラの値をコピー
	// attrsをコピー
	newHandler.attrs = make([]slog.Attr, len(h.attrs))
	copy(newHandler.attrs, h.attrs)
	// groupsをコピーして追加
	newHandler.groups = make([]string, len(h.groups)+1)
	copy(newHandler.groups, h.groups)
	newHandler.groups[len(h.groups)] = name
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
		return fmt.Sprintf("%5s", level.String())
	}
}
