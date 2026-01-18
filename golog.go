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
	"time"

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

// 一般的なタイムフォーマット定数
const (
	defaultTimeFormat = "2006-01-02 15:04:05.000"
)

// timeFormatterFunc は時刻をバッファにフォーマットする関数型
type timeFormatterFunc func(*buffer.Buffer, time.Time)

// formatTimeDefault はデフォルトフォーマット "2006-01-02 15:04:05.000" 用の最適化された関数
func formatTimeDefault(buf *buffer.Buffer, t time.Time) {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	nsec := t.Nanosecond()

	*buf = strconv.AppendInt(*buf, int64(year), 10)
	buf.WriteByte('-')
	if month < 10 {
		buf.WriteByte('0')
	}
	*buf = strconv.AppendInt(*buf, int64(month), 10)
	buf.WriteByte('-')
	if day < 10 {
		buf.WriteByte('0')
	}
	*buf = strconv.AppendInt(*buf, int64(day), 10)
	buf.WriteByte(' ')
	if hour < 10 {
		buf.WriteByte('0')
	}
	*buf = strconv.AppendInt(*buf, int64(hour), 10)
	buf.WriteByte(':')
	if min < 10 {
		buf.WriteByte('0')
	}
	*buf = strconv.AppendInt(*buf, int64(min), 10)
	buf.WriteByte(':')
	if sec < 10 {
		buf.WriteByte('0')
	}
	*buf = strconv.AppendInt(*buf, int64(sec), 10)
	buf.WriteByte('.')

	ms := nsec / 1000000
	if ms < 100 {
		buf.WriteByte('0')
		if ms < 10 {
			buf.WriteByte('0')
		}
	}
	*buf = strconv.AppendInt(*buf, int64(ms), 10)
}

// formatTimeRFC3339 はRFC3339フォーマット用の最適化された関数
func formatTimeRFC3339(buf *buffer.Buffer, t time.Time) {
	*buf = t.AppendFormat(*buf, time.RFC3339)
}

// formatTimeRFC3339Nano はRFC3339Nanoフォーマット用の最適化された関数
func formatTimeRFC3339Nano(buf *buffer.Buffer, t time.Time) {
	*buf = t.AppendFormat(*buf, time.RFC3339Nano)
}

// makeTimeFormatter は指定されたフォーマット文字列に応じた最適な formatter を返す
func makeTimeFormatter(format string) timeFormatterFunc {
	switch format {
	case defaultTimeFormat:
		return formatTimeDefault
	case time.RFC3339:
		return formatTimeRFC3339
	case time.RFC3339Nano:
		return formatTimeRFC3339Nano
	default:
		return func(buf *buffer.Buffer, t time.Time) {
			*buf = t.AppendFormat(*buf, format)
		}
	}
}

// Handler は指定されたフォーマットでログを出力するハンドラー
type Handler struct {
	out               io.Writer
	minLevel          slog.Level
	timeFormat        string
	timeFormatter     timeFormatterFunc
	groups            []string
	useColors         bool
	addSource         bool
	replaceAttr       func(groups []string, a slog.Attr) slog.Attr
	mu                *sync.Mutex
	preformattedAttrs []byte
}

// Options はカスタムハンドラーのオプション
type Options struct {
	Level       slog.Leveler
	UseColors   bool
	TimeFormat  string // 空の場合は "2006-01-02 15:04:05.000" を使用
	AddSource   bool
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

// NewHandler は新しいカスタムハンドラーを作成します
func NewHandler(w io.Writer, opts *Options) *Handler {
	var level slog.Level
	useColors := false
	addSource := false
	var replaceAttr func(groups []string, a slog.Attr) slog.Attr
	timeFormat := "2006-01-02 15:04:05.000"

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
		out:           w,
		minLevel:      level,
		timeFormat:    timeFormat,
		timeFormatter: makeTimeFormatter(timeFormat),
		groups:        []string{},
		useColors:     useColors,
		addSource:     addSource,
		replaceAttr:   replaceAttr,
		mu:            &sync.Mutex{},
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

	buf := buffer.New()
	defer buf.Free()

	timeAttr := slog.Time(slog.TimeKey, r.Time)
	if h.replaceAttr != nil {
		timeAttr = h.replaceAttr(nil, timeAttr)
	}
	if timeAttr.Key != "" {
		buf.WriteByte('[')
		if t, ok := timeAttr.Value.Any().(time.Time); ok {
			h.timeFormatter(buf, t)
		} else {
			h.timeFormatter(buf, r.Time)
		}
		buf.WriteString("] ")
	}

	levelAttr := slog.Any(slog.LevelKey, r.Level)
	if h.replaceAttr != nil {
		levelAttr = h.replaceAttr(nil, levelAttr)
	}
	if levelAttr.Key != "" {
		buf.WriteByte('[')
		var level slog.Level
		if lvl, ok := levelAttr.Value.Any().(slog.Level); ok {
			level = lvl
		} else {
			level = r.Level
		}
		levelStr := h.formatLevelWithColor(level)
		buf.WriteString(levelStr)
		buf.WriteString("] ")
	}

	msgAttr := slog.String(slog.MessageKey, r.Message)
	if h.replaceAttr != nil {
		msgAttr = h.replaceAttr(nil, msgAttr)
	}
	if msgAttr.Key != "" {
		buf.WriteString("msg=")
		if msgErr := formatValue(buf, msgAttr.Value.Any()); msgErr != nil {
			buf.WriteString("\"!ERROR:")
			buf.WriteString(msgErr.Error())
			buf.WriteByte('"')
		}
	}

	if len(h.preformattedAttrs) > 0 {
		buf.Write(h.preformattedAttrs)
	}

	if h.addSource {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			file := filepath.Base(f.File)
			sourceStr := file + ":" + strconv.Itoa(f.Line)

			sourceAttr := slog.String(slog.SourceKey, sourceStr)
			if h.replaceAttr != nil {
				sourceAttr = h.replaceAttr(nil, sourceAttr)
			}
			if sourceAttr.Key != "" {
				buf.WriteString(" ")
				if needsQuoting(sourceAttr.Key) {
					buf.WriteString(strconv.Quote(sourceAttr.Key))
				} else {
					buf.WriteString(sourceAttr.Key)
				}
				buf.WriteString("=")
				formatValue(buf, sourceAttr.Value.Any())
			}
		}
	}

	r.Attrs(func(attr slog.Attr) bool {
		appendAttr(buf, attr.Key, attr.Value, h.groups, h.replaceAttr)
		return true
	})

	buf.WriteByte('\n')

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
		if r <= ' ' || r == '=' || r == '"' || r == 0x7f {
			return true
		}
	}
	return false
}

func appendAttr(buf *buffer.Buffer, key string, value slog.Value, groups []string, replaceAttr func(groups []string, a slog.Attr) slog.Attr) {
	attr := slog.Attr{Key: key, Value: value}
	if replaceAttr != nil {
		attr = replaceAttr(groups, attr)
		if attr.Key == "" {
			return
		}
	}

	buf.WriteByte(' ')

	if len(groups) > 0 {
		for _, group := range groups {
			if needsQuoting(group) {
				buf.WriteString(strconv.Quote(group))
			} else {
				buf.WriteString(group)
			}
			buf.WriteByte('.')
		}
	}

	if needsQuoting(attr.Key) {
		buf.WriteString(strconv.Quote(attr.Key))
	} else {
		buf.WriteString(attr.Key)
	}
	buf.WriteByte('=')
	if err := formatValue(buf, attr.Value.Any()); err != nil {
		buf.WriteString("\"!ERROR:")
		buf.WriteString(err.Error())
		buf.WriteByte('"')
	}
}

// formatLevelWithColor はログレベルを色付きでフォーマットします
func (h *Handler) formatLevelWithColor(level slog.Level) string {
	levelStr := formatLevel(level)

	if !h.useColors {
		return levelStr
	}

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

// formatValue は値を適切な形式に変換してバッファに書き込みます
func formatValue(buf *buffer.Buffer, v any) error {
	if v == nil {
		buf.WriteString("null")
		return nil
	}

	if lv, ok := v.(slog.LogValuer); ok {
		return formatValue(buf, lv.LogValue().Any())
	}

	if s, ok := v.(string); ok {
		buf.WriteString(strconv.Quote(s))
		return nil
	}

	switch v := v.(type) {
	case int:
		*buf = strconv.AppendInt(*buf, int64(v), 10)
		return nil
	case int8:
		*buf = strconv.AppendInt(*buf, int64(v), 10)
		return nil
	case int16:
		*buf = strconv.AppendInt(*buf, int64(v), 10)
		return nil
	case int32:
		*buf = strconv.AppendInt(*buf, int64(v), 10)
		return nil
	case int64:
		*buf = strconv.AppendInt(*buf, v, 10)
		return nil
	case uint:
		*buf = strconv.AppendUint(*buf, uint64(v), 10)
		return nil
	case uint8:
		*buf = strconv.AppendUint(*buf, uint64(v), 10)
		return nil
	case uint16:
		*buf = strconv.AppendUint(*buf, uint64(v), 10)
		return nil
	case uint32:
		*buf = strconv.AppendUint(*buf, uint64(v), 10)
		return nil
	case uint64:
		*buf = strconv.AppendUint(*buf, v, 10)
		return nil
	case float32:
		*buf = strconv.AppendFloat(*buf, float64(v), 'f', -1, 32)
		return nil
	case float64:
		*buf = strconv.AppendFloat(*buf, v, 'f', -1, 64)
		return nil
	case bool:
		*buf = strconv.AppendBool(*buf, v)
		return nil
	case LogFormatter:
		s, err := v.FormatForLog()
		if err != nil {
			return err
		}
		buf.WriteString(s)
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		buf.WriteString("null")
		return nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf.Write(b)
	return nil
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

	newHandler := *h

	newHandler.groups = make([]string, len(h.groups))
	copy(newHandler.groups, h.groups)

	buf := buffer.New()
	defer buf.Free()

	if len(h.preformattedAttrs) > 0 {
		buf.Write(h.preformattedAttrs)
	}

	for _, attr := range attrs {
		appendAttr(buf, attr.Key, attr.Value, h.groups, h.replaceAttr)
	}

	newHandler.preformattedAttrs = make([]byte, buf.Len())
	copy(newHandler.preformattedAttrs, *buf)

	return &newHandler
}

// WithGroup は新しいグループを持つハンドラーを返します
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	newHandler := *h

	if len(h.preformattedAttrs) > 0 {
		newHandler.preformattedAttrs = make([]byte, len(h.preformattedAttrs))
		copy(newHandler.preformattedAttrs, h.preformattedAttrs)
	}

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
		return " INFO"
	case slog.LevelWarn:
		return " WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		s := level.String()
		if len(s) < 5 {
			return strings.Repeat(" ", 5-len(s)) + s
		}
		return s
	}
}
