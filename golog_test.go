package loggo

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/f0reth/golog/internal/buffer"
)

// TestNewHandler は NewHandler の初期化をテストします
func TestNewHandler(t *testing.T) {
	t.Run("nil options", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, nil)
		if handler == nil {
			t.Fatal("handler should not be nil")
		}
		if handler.minLevel != slog.LevelInfo {
			t.Errorf("expected default level to be Info (0), got %d", handler.minLevel)
		}
		if handler.useColors {
			t.Error("expected useColors to be false by default")
		}
	})

	t.Run("with options", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelDebug,
			UseColors: true,
		})
		if handler.minLevel != slog.LevelDebug {
			t.Errorf("expected level to be Debug, got %d", handler.minLevel)
		}
		if !handler.useColors {
			t.Error("expected useColors to be true")
		}
	})
}

// TestEnabled は Enabled メソッドをテストします
func TestEnabled(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level: slog.LevelWarn,
	})

	ctx := context.Background()
	if handler.Enabled(ctx, slog.LevelDebug) {
		t.Error("Debug should be disabled when min level is Warn")
	}
	if handler.Enabled(ctx, slog.LevelInfo) {
		t.Error("Info should be disabled when min level is Warn")
	}
	if !handler.Enabled(ctx, slog.LevelWarn) {
		t.Error("Warn should be enabled when min level is Warn")
	}
	if !handler.Enabled(ctx, slog.LevelError) {
		t.Error("Error should be enabled when min level is Warn")
	}
}

// TestHandle は基本的なログ出力をテストします
func TestHandle(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("output should contain the message")
	}
	if !strings.Contains(output, "key=\"value\"") {
		t.Error("output should contain the attribute")
	}
	if !strings.Contains(output, "INFO") {
		t.Error("output should contain the level")
	}
}

// TestLogLevels は各ログレベルの出力をテストします
func TestLogLevels(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, " INFO"},
		{slog.LevelWarn, " WARN"},
		{slog.LevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewHandler(&buf, &Options{
				Level:     slog.LevelDebug,
				UseColors: false,
			})

			logger := slog.New(handler)
			logger.Log(context.Background(), tt.level, "test")

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("expected output to contain %q, got %q", tt.expected, output)
			}
		})
	}
}

// TestWithAttrs は WithAttrs メソッドをテストします
func TestWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger = logger.With("context", "test")
	logger.Info("message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "context=\"test\"") {
		t.Error("output should contain the context attribute")
	}
	if !strings.Contains(output, "key=\"value\"") {
		t.Error("output should contain the key attribute")
	}
}

// TestWithGroup は WithGroup メソッドをテストします
func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.WithGroup("db").Info("query executed", "sql", "SELECT * FROM users")

	output := buf.String()
	if !strings.Contains(output, "db.sql=\"SELECT * FROM users\"") {
		t.Errorf("output should contain grouped attribute, got: %s", output)
	}
}

// TestNestedGroups はネストされたグループをテストします
func TestNestedGroups(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.WithGroup("service").WithGroup("db").Info("query", "table", "users")

	output := buf.String()
	if !strings.Contains(output, "service.db.table=\"users\"") {
		t.Errorf("output should contain nested groups, got: %s", output)
	}
}

// TestGroupWithAttrs はグループと属性の組み合わせをテストします
func TestGroupWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger = logger.WithGroup("server").With("port", 8080)
	logger.Info("started")

	output := buf.String()
	if !strings.Contains(output, "server.port=8080") {
		t.Errorf("output should contain grouped attribute from WithAttrs, got: %s", output)
	}
}

// TestColors はカラー出力をテストします
func TestColors(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: true,
	})

	logger := slog.New(handler)
	logger.Info("test")

	output := buf.String()
	if !strings.Contains(output, colorGreen) {
		t.Error("output should contain color codes when colors are enabled")
	}
	if !strings.Contains(output, colorReset) {
		t.Error("output should contain color reset code")
	}
}

// TestFormatValue は formatValue 関数をテストします
func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
		hasError bool
	}{
		{"nil", nil, "null", false},
		{"string", "hello", `"hello"`, false},
		{"int", 42, "42", false},
		{"float", 3.14, "3.14", false},
		{"bool", true, "true", false},
		{"escaped string", `hello"world`, `"hello\"world"`, false},
		{"newline", "line1\nline2", `"line1\nline2"`, false},
		{"tab", "hello\tworld", `"hello\tworld"`, false},
		{"carriage return", "hello\rworld", `"hello\rworld"`, false},
		{"backslash", `hello\world`, `"hello\\world"`, false},
		// ASCII制御文字のテスト
		{"null byte", "hello\x00world", `"hello\x00world"`, false},
		{"bell", "hello\x07world", `"hello\aworld"`, false},
		{"backspace", "hello\x08world", `"hello\bworld"`, false},
		{"form feed", "hello\x0cworld", `"hello\fworld"`, false},
		{"vertical tab", "hello\x0bworld", `"hello\vworld"`, false},
		{"control chars", "\x01\x02\x03", `"\x01\x02\x03"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := buffer.New()
			defer buf.Free()
			err := formatValue(buf, tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("expected error=%v, got error=%v", tt.hasError, err)
			}
			result := string(*buf)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestNilPointer は nil ポインタの処理をテストします
func TestNilPointer(t *testing.T) {
	type TestStruct struct {
		Value string
	}

	var nilPtr *TestStruct
	formatBuf := buffer.New()
	defer formatBuf.Free()
	err := formatValue(formatBuf, nilPtr)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	result := string(*formatBuf)
	if result != "null" {
		t.Errorf("expected \"null\", got %q", result)
	}

	// ログ出力でもテスト
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("test", "ptr", nilPtr)

	output := buf.String()
	if !strings.Contains(output, "ptr=null") {
		t.Errorf("expected output to contain ptr=null, got: %s", output)
	}
}

// CustomType は LogFormatter を実装するテスト用の型です
type CustomType struct {
	Value string
}

// FormatForLog は LogFormatter インターフェースを実装します
func (c CustomType) FormatForLog() (string, error) {
	return `"custom:` + c.Value + `"`, nil
}

// TestLogFormatter は LogFormatter インターフェースをテストします
func TestLogFormatter(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("test", "custom", CustomType{Value: "test"})

	output := buf.String()
	if !strings.Contains(output, `custom="custom:test"`) {
		t.Errorf("output should use LogFormatter, got: %s", output)
	}
}

// UserID は slog.LogValuer を実装するテスト用の型です
type UserID int

func (u UserID) LogValue() slog.Value {
	return slog.StringValue("user_" + strconv.Itoa(int(u)))
}

// SensitiveData は機密情報をマスクするテスト用の型です
type SensitiveData struct {
	Secret string
}

func (s SensitiveData) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

// NestedLogValuer は別の LogValuer を返すテスト用の型です
type NestedLogValuer struct {
	ID UserID
}

func (n NestedLogValuer) LogValue() slog.Value {
	return slog.AnyValue(n.ID)
}

// IntLogValuer は整数を文字列として返すテスト用の型です
type IntLogValuer int

func (i IntLogValuer) LogValue() slog.Value {
	return slog.IntValue(int(i) * 10)
}

// TestLogValuer は slog.LogValuer インターフェースをテストします
func TestLogValuer(t *testing.T) {
	t.Run("basic LogValuer", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
		})

		logger := slog.New(handler)
		logger.Info("test", "user_id", UserID(12345))

		output := buf.String()
		if !strings.Contains(output, `user_id="user_12345"`) {
			t.Errorf("output should use LogValuer, got: %s", output)
		}
	})

	t.Run("sensitive data masking", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
		})

		logger := slog.New(handler)
		logger.Info("test", "password", SensitiveData{Secret: "secret123"})

		output := buf.String()
		if !strings.Contains(output, `password="[REDACTED]"`) {
			t.Errorf("output should mask sensitive data, got: %s", output)
		}
		if strings.Contains(output, "secret123") {
			t.Error("output should not contain secret value")
		}
	})

	t.Run("nested LogValuer", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
		})

		logger := slog.New(handler)
		logger.Info("test", "nested", NestedLogValuer{ID: UserID(999)})

		output := buf.String()
		if !strings.Contains(output, `nested="user_999"`) {
			t.Errorf("output should resolve nested LogValuer, got: %s", output)
		}
	})

	t.Run("LogValuer returning int", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
		})

		logger := slog.New(handler)
		// IntLogValuer(5) -> 50 に変換される
		logger.Info("test", "multiplied", IntLogValuer(5))

		output := buf.String()
		if !strings.Contains(output, "multiplied=50") {
			t.Errorf("output should contain multiplied=50, got: %s", output)
		}
	})
}

// DualFormatter は LogValuer と LogFormatter の両方を実装する型です
type DualFormatter struct {
	Value string
}

// LogValue は slog.LogValuer インターフェースを実装します
// LogValuer は LogFormatter より優先される
func (d DualFormatter) LogValue() slog.Value {
	return slog.StringValue("logvaluer:" + d.Value)
}

// FormatForLog は LogFormatter インターフェースを実装します
func (d DualFormatter) FormatForLog() (string, error) {
	return `"formatter:` + d.Value + `"`, nil
}

// TestLogValuerWithFormatter は LogValuer と LogFormatter の優先順位をテストします
func TestLogValuerWithFormatter(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("test", "dual", DualFormatter{Value: "test"})

	output := buf.String()
	// LogValuer が優先されるべき
	if !strings.Contains(output, `dual="logvaluer:test"`) {
		t.Errorf("LogValuer should take precedence, got: %s", output)
	}
	if strings.Contains(output, "formatter:test") {
		t.Error("LogFormatter should not be used when LogValuer is present")
	}
}

// TestTimeFormat は時刻フォーマットをテストします
func TestTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	ctx := context.Background()
	record := slog.NewRecord(time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC), slog.LevelInfo, "test", 0)
	handler.Handle(ctx, record)

	output := buf.String()
	// ミリ秒までの時刻フォーマットを確認
	if !strings.Contains(output, "2024-01-15 10:30:45.123") {
		t.Errorf("expected time format with milliseconds, got: %s", output)
	}
}

// TestCustomTimeFormat はカスタム時刻フォーマットをテストします
func TestCustomTimeFormat(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		expected   string
		timeToTest time.Time
	}{
		{
			name:       "RFC3339",
			format:     time.RFC3339,
			expected:   "2024-01-15T10:30:45Z",
			timeToTest: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:       "only date",
			format:     "2006-01-02",
			expected:   "2024-01-15",
			timeToTest: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:       "only time",
			format:     "15:04:05",
			expected:   "10:30:45",
			timeToTest: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:       "Unix timestamp",
			format:     time.UnixDate,
			expected:   "Mon Jan 15 10:30:45 UTC 2024",
			timeToTest: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
		{
			name:       "custom format",
			format:     "2006/01/02 15:04:05",
			expected:   "2024/01/15 10:30:45",
			timeToTest: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewHandler(&buf, &Options{
				Level:      slog.LevelInfo,
				UseColors:  false,
				TimeFormat: tt.format,
			})

			ctx := context.Background()
			record := slog.NewRecord(tt.timeToTest, slog.LevelInfo, "test", 0)
			handler.Handle(ctx, record)

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("expected output to contain %q, got: %s", tt.expected, output)
			}
		})
	}
}

// TestDefaultTimeFormat はデフォルトの時刻フォーマットをテストします
func TestDefaultTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	// TimeFormatを指定しない（デフォルトを使用）
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	ctx := context.Background()
	record := slog.NewRecord(time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC), slog.LevelInfo, "test", 0)
	handler.Handle(ctx, record)

	output := buf.String()
	// デフォルトのミリ秒までのフォーマットを確認
	if !strings.Contains(output, "2024-01-15 10:30:45.123") {
		t.Errorf("expected default time format with milliseconds, got: %s", output)
	}
}

// TestEmptyTimeFormat は空文字列のTimeFormatでデフォルトが使用されることをテストします
func TestEmptyTimeFormat(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:      slog.LevelInfo,
		UseColors:  false,
		TimeFormat: "", // 空文字列を明示的に指定
	})

	ctx := context.Background()
	record := slog.NewRecord(time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC), slog.LevelInfo, "test", 0)
	handler.Handle(ctx, record)

	output := buf.String()
	// デフォルトのミリ秒までのフォーマットが使用されるはず
	if !strings.Contains(output, "2024-01-15 10:30:45.123") {
		t.Errorf("expected default time format when empty string is provided, got: %s", output)
	}
}

// TestConcurrentWrites は並行書き込みのテストです
func TestConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	const goroutines = 100
	const iterations = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range iterations {
				logger.Info("concurrent test", "goroutine", id, "iteration", i)
			}
		}(g)
	}
	wg.Wait()

	// レースコンディションが無ければテスト成功
	// （-race フラグでテストすることでレースコンディションを検出可能）
}

// TestWithAttrsEmpty は空の属性配列での WithAttrs をテストします
func TestWithAttrsEmpty(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	// 空の属性配列を渡す
	newHandler := handler.WithAttrs([]slog.Attr{})

	// 元のハンドラーと同じインスタンスが返されるべき
	if newHandler != handler {
		t.Error("WithAttrs with empty slice should return the same handler")
	}
}

// TestWithAttrsMultiple は複数回 WithAttrs を呼んだ場合をテストします
func TestWithAttrsMultiple(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger = logger.With("first", "1").With("second", "2").With("third", "3")
	logger.Info("test")

	output := buf.String()
	if !strings.Contains(output, "first=\"1\"") {
		t.Error("output should contain first attribute")
	}
	if !strings.Contains(output, "second=\"2\"") {
		t.Error("output should contain second attribute")
	}
	if !strings.Contains(output, "third=\"3\"") {
		t.Error("output should contain third attribute")
	}
}

// TestWithAttrsAfterWithGroup は WithGroup の後に WithAttrs を呼んだ場合をテストします
func TestWithAttrsAfterWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger = logger.With("before", "group")
	logger = logger.WithGroup("g1").With("inside", "group")
	logger.Info("test", "after", "log")

	output := buf.String()
	if !strings.Contains(output, "before=\"group\"") {
		t.Errorf("output should contain attribute before group, got: %s", output)
	}
	if !strings.Contains(output, "g1.inside=\"group\"") {
		t.Errorf("output should contain grouped attribute, got: %s", output)
	}
	if !strings.Contains(output, "g1.after=\"log\"") {
		t.Errorf("output should contain attribute in group, got: %s", output)
	}
}

// TestComplexStructures は複雑な構造体のログ出力をテストします
func TestComplexStructures(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	// スライス
	logger.Info("slice test", "numbers", []int{1, 2, 3})
	output := buf.String()
	if !strings.Contains(output, "numbers=[1,2,3]") {
		t.Errorf("output should contain slice, got: %s", output)
	}

	buf.Reset()

	// マップ
	logger.Info("map test", "data", map[string]int{"a": 1, "b": 2})
	output = buf.String()
	// マップの順序は不定なので、キーの存在をチェック
	if !strings.Contains(output, `"a"`) || !strings.Contains(output, `"b"`) {
		t.Errorf("output should contain map keys, got: %s", output)
	}

	buf.Reset()

	// 構造体
	type Person struct {
		Name string
		Age  int
	}
	logger.Info("struct test", "person", Person{Name: "Alice", Age: 30})
	output = buf.String()
	if !strings.Contains(output, `"Name":"Alice"`) || !strings.Contains(output, `"Age":30`) {
		t.Errorf("output should contain struct fields, got: %s", output)
	}
}

// TestLongString は非常に長い文字列のテストです
func TestLongString(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	// 1000文字の文字列
	longStr := strings.Repeat("a", 1000)
	logger.Info("long string test", "data", longStr)

	output := buf.String()
	if !strings.Contains(output, longStr) {
		t.Error("output should contain the long string")
	}
}

// TestManyAttributes は大量の属性のテストです
func TestManyAttributes(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	// 50個の属性
	attrs := make([]any, 100) // key-value pairs
	for i := 0; i < 50; i++ {
		attrs[i*2] = "key" + string(rune('0'+i%10))
		attrs[i*2+1] = i
	}

	logger.Info("many attributes test", attrs...)

	output := buf.String()
	// いくつかの属性が含まれているか確認
	if !strings.Contains(output, "key0") || !strings.Contains(output, "key5") {
		t.Errorf("output should contain attributes, got: %s", output)
	}
}

// TestEmptyString は空文字列のテストです
func TestEmptyString(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("", "key", "")

	output := buf.String()
	if !strings.Contains(output, `msg=""`) {
		t.Error("output should handle empty message")
	}
	if !strings.Contains(output, `key=""`) {
		t.Error("output should handle empty attribute value")
	}
}

// TestCustomLogLevel はカスタムログレベルのテストです
func TestCustomLogLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	// カスタムログレベル (Error + 4)
	customLevel := slog.LevelError + 4
	logger.Log(context.Background(), customLevel, "custom level test")

	output := buf.String()
	// カスタムレベルが5文字幅で出力されることを確認
	if !strings.Contains(output, "ERROR+4") && !strings.Contains(output, "12") {
		t.Errorf("output should contain custom level, got: %s", output)
	}
}

// ErrorFormatter は FormatForLog でエラーを返すテスト用の型です
type ErrorFormatter struct{}

func (e ErrorFormatter) FormatForLog() (string, error) {
	return "", context.DeadlineExceeded
}

// TestLogFormatterError は LogFormatter がエラーを返す場合をテストします
func TestLogFormatterError(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("test", "error_formatter", ErrorFormatter{})

	output := buf.String()
	if !strings.Contains(output, "!ERROR:") {
		t.Errorf("output should contain error marker, got: %s", output)
	}
	if !strings.Contains(output, "context deadline exceeded") {
		t.Errorf("output should contain error message, got: %s", output)
	}
}

// TestAllColorLevels はすべてのログレベルの色をテストします
func TestAllColorLevels(t *testing.T) {
	tests := []struct {
		level slog.Level
		color string
	}{
		{slog.LevelDebug, colorCyan},
		{slog.LevelInfo, colorGreen},
		{slog.LevelWarn, colorYellow},
		{slog.LevelError, colorRed},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewHandler(&buf, &Options{
				Level:     slog.LevelDebug,
				UseColors: true,
			})

			logger := slog.New(handler)
			logger.Log(context.Background(), tt.level, "test")

			output := buf.String()
			if !strings.Contains(output, tt.color) {
				t.Errorf("output should contain color %s, got: %s", tt.color, output)
			}
		})
	}
}

// TestVariousNumericTypes は様々な数値型のテストです
func TestVariousNumericTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{"int8", int8(127), "127"},
		{"int16", int16(32767), "32767"},
		{"int32", int32(2147483647), "2147483647"},
		{"int64", int64(9223372036854775807), "9223372036854775807"},
		{"uint", uint(42), "42"},
		{"uint8", uint8(255), "255"},
		{"uint16", uint16(65535), "65535"},
		{"uint32", uint32(4294967295), "4294967295"},
		{"uint64", uint64(18446744073709551615), "18446744073709551615"},
		{"float32", float32(3.14), "3.14"},
		{"float64", float64(2.718281828), "2.718281828"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := buffer.New()
			defer buf.Free()
			err := formatValue(buf, tt.value)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			result := string(*buf)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestHandlerIndependence は複数のハンドラーの独立性をテストします
func TestHandlerIndependence(t *testing.T) {
	var buf bytes.Buffer
	handler1 := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger1 := slog.New(handler1)
	logger2 := logger1.With("handler", "2")

	logger1.Info("from handler1")
	logger2.Info("from handler2")

	output1 := buf.String()
	lines := strings.Split(strings.TrimSpace(output1), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}

	// 最初のログには "handler" 属性がないはず
	if strings.Contains(lines[0], "handler=") {
		t.Errorf("first log should not have handler attribute, got: %s", lines[0])
	}

	// 2番目のログには "handler" 属性があるはず
	if !strings.Contains(lines[1], "handler=\"2\"") {
		t.Errorf("second log should have handler attribute, got: %s", lines[1])
	}
}

// TestBufferPoolReuse はBuffer Poolの再利用をテストします
func TestBufferPoolReuse(t *testing.T) {
	// Buffer Poolから2つのバッファを取得
	buf1 := buffer.New()
	buf1.WriteString("test1")
	ptr1 := &(*buf1)[0] // 最初のバッファのアドレスを保存

	// バッファをプールに戻す
	buf1.Free()

	// 新しいバッファを取得（同じバッファが再利用されるはず）
	buf2 := buffer.New()

	// バッファがリセットされていることを確認
	if buf2.Len() != 0 {
		t.Errorf("reused buffer should be empty, got length %d", buf2.Len())
	}

	// 同じバッファが再利用されたか確認（ポインタの比較）
	if len(*buf2) > 0 {
		ptr2 := &(*buf2)[0]
		if ptr1 != ptr2 {
			// 常に同じではないが、多くの場合再利用される
			t.Logf("buffer was not reused (this is not necessarily an error)")
		}
	}

	buf2.Free()
}

// TestBufferPoolLargeBuffer は大きなバッファがプールに戻されないことをテストします
func TestBufferPoolLargeBuffer(t *testing.T) {
	buf := buffer.New()

	// 16KB + 1バイトの大きなデータを書き込む
	largeData := make([]byte, 16*1024+1)
	for i := range largeData {
		largeData[i] = 'a'
	}
	buf.Write(largeData)

	// 容量が16KBを超えていることを確認
	if cap(*buf) <= 16*1024 {
		t.Errorf("buffer capacity should exceed 16KB, got %d", cap(*buf))
	}

	// Free を呼んでも、大きすぎるバッファはプールに戻されない
	buf.Free()

	// 新しいバッファを取得（通常サイズのバッファが返されるはず）
	buf2 := buffer.New()
	if cap(*buf2) > 16*1024 {
		t.Errorf("new buffer should not have large capacity, got %d", cap(*buf2))
	}
	buf2.Free()
}

// TestBufferOperations はBuffer の基本操作をテストします
func TestBufferOperations(t *testing.T) {
	buf := buffer.New()
	defer buf.Free()

	// WriteString
	buf.WriteString("hello")
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}

	// WriteByte
	buf.WriteByte(' ')
	if buf.String() != "hello " {
		t.Errorf("expected 'hello ', got %q", buf.String())
	}

	// Write
	buf.Write([]byte("world"))
	if buf.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", buf.String())
	}

	// Len
	if buf.Len() != 11 {
		t.Errorf("expected length 11, got %d", buf.Len())
	}

	// Reset
	buf.Reset()
	if buf.Len() != 0 {
		t.Errorf("expected length 0 after reset, got %d", buf.Len())
	}

	// SetLen
	buf.WriteString("hello world")
	buf.SetLen(5)
	if buf.String() != "hello" {
		t.Errorf("expected 'hello' after SetLen, got %q", buf.String())
	}
}

// TestDisabledLevel はログレベルによる出力の抑制をテストします
func TestDisabledLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelWarn,
		UseColors: false,
	})

	logger := slog.New(handler)

	// DEBUGとINFOは出力されないはず
	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Errorf("no output expected for disabled levels, got: %s", buf.String())
	}

	// WARNとERRORは出力されるはず
	logger.Warn("warn message")
	output := buf.String()
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should be logged")
	}
}

// TestNilValue はnil値のテストです
func TestNilValue(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	// nil interface
	var nilInterface any
	logger.Info("test", "nil_value", nilInterface)

	output := buf.String()
	if !strings.Contains(output, "nil_value=null") {
		t.Errorf("output should contain null for nil value, got: %s", output)
	}
}

// TestStructWithNilPointer はnil ポインタを含む構造体のテストです
func TestStructWithNilPointer(t *testing.T) {
	type Inner struct {
		Value string
	}
	type Outer struct {
		Ptr *Inner
	}

	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("test", "data", Outer{Ptr: nil})

	output := buf.String()
	if !strings.Contains(output, "Ptr") && !strings.Contains(output, "null") {
		t.Errorf("output should handle nil pointer in struct, got: %s", output)
	}
}

// discardWriter は書き込みを破棄する io.Writer です
type discardWriter struct{}

func (d discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// TestHighVolumeLogging は大量のログ出力でメモリリークがないかテストします
func TestHighVolumeLogging(t *testing.T) {
	handler := NewHandler(discardWriter{}, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	// 10000回のログ出力
	for i := range 10000 {
		logger.Info("high volume test", "iteration", i, "data", "some data")
	}

	// メモリリークがなければテストパス
	// （実際のメモリリークテストは -memprofile で確認）
}

// TestAttributeOrder は属性の順序が保持されることをテストします
func TestAttributeOrder(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)
	logger.Info("test", "first", "1", "second", "2", "third", "3")

	output := buf.String()

	// 属性が順序通りに出力されているか確認
	firstIdx := strings.Index(output, "first")
	secondIdx := strings.Index(output, "second")
	thirdIdx := strings.Index(output, "third")

	if firstIdx == -1 || secondIdx == -1 || thirdIdx == -1 {
		t.Error("all attributes should be present")
	}

	if !(firstIdx < secondIdx && secondIdx < thirdIdx) {
		t.Errorf("attributes should be in order: first(%d), second(%d), third(%d)", firstIdx, secondIdx, thirdIdx)
	}
}

// TestPreformattedAttrsWithMultipleWithAttrs は複数のWithAttrsで事前フォーマットをテストします
func TestPreformattedAttrsWithMultipleWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	// 複数回WithAttrsを呼ぶ
	h1 := handler.WithAttrs([]slog.Attr{slog.String("a", "1")})
	h2 := h1.WithAttrs([]slog.Attr{slog.String("b", "2")})
	h3 := h2.WithAttrs([]slog.Attr{slog.String("c", "3")})

	logger := slog.New(h3)
	logger.Info("test")

	output := buf.String()

	// すべての属性が含まれているか確認
	if !strings.Contains(output, `a="1"`) {
		t.Errorf("output should contain a=1, got: %s", output)
	}
	if !strings.Contains(output, `b="2"`) {
		t.Errorf("output should contain b=2, got: %s", output)
	}
	if !strings.Contains(output, `c="3"`) {
		t.Errorf("output should contain c=3, got: %s", output)
	}
}

// TestAddSource はAddSourceオプションがソースファイルと行番号を追加することをテストします
func TestAddSource(t *testing.T) {
	t.Run("AddSource disabled", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
			AddSource: false,
		})

		logger := slog.New(handler)
		logger.Info("test message")

		output := buf.String()
		if strings.Contains(output, "source=") {
			t.Errorf("output should not contain source when AddSource is false, got: %s", output)
		}
	})

	t.Run("AddSource enabled", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
			AddSource: true,
		})

		logger := slog.New(handler)
		logger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, "source=") {
			t.Errorf("output should contain source when AddSource is true, got: %s", output)
		}

		// ソース情報にファイル名と行番号が含まれているか確認
		if !strings.Contains(output, "golog_test.go:") {
			t.Errorf("output should contain source file name and line number, got: %s", output)
		}
	})

	t.Run("AddSource with WithAttrs", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
			AddSource: true,
		})

		// WithAttrsでaddSourceが保持されることを確認
		h := handler.WithAttrs([]slog.Attr{slog.String("key", "value")})
		logger := slog.New(h)
		logger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, "source=") {
			t.Errorf("output should contain source when AddSource is true with WithAttrs, got: %s", output)
		}
		if !strings.Contains(output, `key="value"`) {
			t.Errorf("output should contain the attribute, got: %s", output)
		}
	})

	t.Run("AddSource with WithGroup", func(t *testing.T) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
			AddSource: true,
		})

		// WithGroupでaddSourceが保持されることを確認
		h := handler.WithGroup("group1")
		logger := slog.New(h)
		logger.Info("test message", "key", "value")

		output := buf.String()
		if !strings.Contains(output, "source=") {
			t.Errorf("output should contain source when AddSource is true with WithGroup, got: %s", output)
		}
		if !strings.Contains(output, `group1.key="value"`) {
			t.Errorf("output should contain the grouped attribute, got: %s", output)
		}
	})
}

// TestReplaceAttr はReplaceAttrコールバックが正しく動作することをテストします
func TestReplaceAttr(t *testing.T) {
	tests := []struct {
		name        string
		replaceAttr func([]string, slog.Attr) slog.Attr
		logFunc     func(*slog.Logger)
		setupOpts   func(*Options)
		want        []string
		notWant     []string
	}{
		{
			name:        "nil (default behavior)",
			replaceAttr: nil,
			logFunc:     func(l *slog.Logger) { l.Info("test", "key", "value") },
			want:        []string{`key="value"`},
		},
		{
			name: "redact password",
			replaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == "password" {
					return slog.String("password", "***REDACTED***")
				}
				return a
			},
			logFunc: func(l *slog.Logger) { l.Info("login", "user", "alice", "password", "secret123") },
			want:    []string{`password="***REDACTED***"`, `user="alice"`},
			notWant: []string{"secret123"},
		},
		{
			name: "remove attribute",
			replaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == "internal" {
					return slog.Attr{}
				}
				return a
			},
			logFunc: func(l *slog.Logger) { l.Info("test", "public", "data", "internal", "secret") },
			want:    []string{`public="data"`},
			notWant: []string{"internal"},
		},
		{
			name: "rename key",
			replaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == "userId" {
					return slog.Attr{Key: "user_id", Value: a.Value}
				}
				return a
			},
			logFunc: func(l *slog.Logger) { l.Info("test", "userId", "12345") },
			want:    []string{`user_id="12345"`},
			notWant: []string{"userId"},
		},
		{
			name: "modify built-in attributes",
			replaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.MessageKey {
					return slog.String(slog.MessageKey, strings.ToUpper(a.Value.String()))
				}
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
			logFunc: func(l *slog.Logger) { l.Info("hello world") },
			want:    []string{"HELLO WORLD", "[ INFO]"},
		},
		{
			name: "with groups",
			replaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if len(groups) > 0 && a.Key == "secret" {
					return slog.String("secret", "***")
				}
				return a
			},
			logFunc: func(l *slog.Logger) { l.WithGroup("auth").Info("test", "secret", "password123") },
			want:    []string{`auth.secret="***"`},
			notWant: []string{"password123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := &Options{
				Level:       slog.LevelInfo,
				UseColors:   false,
				ReplaceAttr: tt.replaceAttr,
			}
			if tt.setupOpts != nil {
				tt.setupOpts(opts)
			}

			handler := NewHandler(&buf, opts)
			logger := slog.New(handler)
			tt.logFunc(logger)

			output := buf.String()
			for _, w := range tt.want {
				if !strings.Contains(output, w) {
					t.Errorf("output should contain %q, got: %s", w, output)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(output, nw) {
					t.Errorf("output should not contain %q, got: %s", nw, output)
				}
			}
		})
	}
}

// TestReplaceAttrModifiesBuiltInValues はReplaceAttrが組み込み属性の値を変更することをテストします
func TestReplaceAttrModifiesBuiltInValues(t *testing.T) {
	t.Run("modify time and level values", func(t *testing.T) {
		var buf bytes.Buffer
		fixedTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		handler := NewHandler(&buf, &Options{
			Level:     slog.LevelInfo,
			UseColors: false,
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Time(slog.TimeKey, fixedTime)
				}
				if a.Key == slog.LevelKey {
					if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == slog.LevelInfo {
						return slog.Any(slog.LevelKey, slog.LevelWarn)
					}
				}
				return a
			},
		})

		logger := slog.New(handler)
		logger.Info("test")

		output := buf.String()
		if !strings.Contains(output, "2000-01-01 00:00:00.000") {
			t.Errorf("output should contain modified time, got: %s", output)
		}
		if !strings.Contains(output, "WARN") {
			t.Errorf("output should contain modified level WARN, got: %s", output)
		}
		if strings.Contains(output, "INFO") {
			t.Errorf("output should not contain original level INFO, got: %s", output)
		}
	})
}

// TestKeyEscaping はキーのエスケープ処理をテストします
func TestKeyEscaping(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*slog.Logger)
		want   string
		unwant string
	}{
		{"space", func(l *slog.Logger) { l.Info("test", "my key", "value") }, `"my key"="value"`, ""},
		{"quote", func(l *slog.Logger) { l.Info("test", `key"name`, "value") }, `"key\"name"="value"`, ""},
		{"equals", func(l *slog.Logger) { l.Info("test", "key=name", "value") }, `"key=name"="value"`, ""},
		{"normal", func(l *slog.Logger) { l.Info("test", "normalKey", "value") }, `normalKey="value"`, `"normalKey"`},
		{"newline", func(l *slog.Logger) { l.Info("test", "key\nname", "value") }, `"key\nname"="value"`, ""},
		{"tab", func(l *slog.Logger) { l.Info("test", "key\tname", "value") }, `"key\tname"="value"`, ""},
		{"empty", func(l *slog.Logger) { l.Info("test", "", "value") }, `""="value"`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewHandler(&buf, &Options{Level: slog.LevelInfo, UseColors: false})
			tt.setup(slog.New(handler))

			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("want %q in output, got: %s", tt.want, output)
			}
			if tt.unwant != "" && strings.Contains(output, tt.unwant) {
				t.Errorf("don't want %q in output, got: %s", tt.unwant, output)
			}
		})
	}

	t.Run("groups with special chars", func(t *testing.T) {
		var buf bytes.Buffer
		h := NewHandler(&buf, &Options{Level: slog.LevelInfo, UseColors: false}).
			WithGroup("group1").WithGroup("group 2").WithGroup("group=3")
		slog.New(h).Info("test", "key", "value")

		if !strings.Contains(buf.String(), `group1."group 2"."group=3".key="value"`) {
			t.Errorf("want quoted group names, got: %s", buf.String())
		}
	})
}

// TestWithGroupEmptyName は空文字列のグループ名が無視されることをテストします
func TestWithGroupEmptyName(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *Handler) slog.Handler
		want  string
	}{
		{"single empty", func(h *Handler) slog.Handler { return h.WithGroup("") }, `key="value"`},
		{"between groups", func(h *Handler) slog.Handler {
			return h.WithGroup("group1").WithGroup("").WithGroup("group2")
		}, `group1.group2.key="value"`},
		{"multiple empty", func(h *Handler) slog.Handler {
			return h.WithGroup("").WithGroup("").WithGroup("")
		}, `key="value"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewHandler(&buf, &Options{Level: slog.LevelInfo, UseColors: false})
			slog.New(tt.setup(h)).Info("test", "key", "value")

			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("want %q in output, got: %s", tt.want, buf.String())
			}
		})
	}

	t.Run("returns same instance", func(t *testing.T) {
		h := NewHandler(&bytes.Buffer{}, &Options{Level: slog.LevelInfo})
		if h.WithGroup("") != h {
			t.Error("WithGroup(\"\") should return the same handler instance")
		}
	})
}

// TestTimeFormatterOptimization は時刻フォーマットの最適化をテストします
func TestTimeFormatterOptimization(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC)

	t.Run("formatTimeDefault", func(t *testing.T) {
		buf := buffer.New()
		defer buf.Free()
		formatTimeDefault(buf, testTime)
		if string(*buf) != "2024-01-15 10:30:45.123" {
			t.Errorf("want 2024-01-15 10:30:45.123, got %s", string(*buf))
		}
	})

	t.Run("makeTimeFormatter", func(t *testing.T) {
		tests := []struct {
			format string
			want   string
		}{
			{"2006-01-02 15:04:05.000", "2024-01-15"},
			{time.RFC3339, "2024-01-15T"},
			{"15:04:05", "10:30:45"},
		}
		for _, tt := range tests {
			buf := buffer.New()
			makeTimeFormatter(tt.format)(buf, testTime)
			result := string(*buf)
			buf.Free()
			if !strings.Contains(result, tt.want) {
				t.Errorf("format %q: want %q in result, got %q", tt.format, tt.want, result)
			}
		}
	})
}

// TestProductionScenarios は実運用シナリオをテストします
func TestProductionScenarios(t *testing.T) {
	t.Run("file write", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "golog_test_*.log")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		handler := NewHandler(tmpFile, &Options{Level: slog.LevelInfo, UseColors: false})
		logger := slog.New(handler)
		logger.Info("production test", "request_id", "12345", "user", "alice")

		tmpFile.Sync()
		data, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			t.Fatal(err)
		}

		output := string(data)
		if !strings.Contains(output, "production test") || !strings.Contains(output, `request_id="12345"`) {
			t.Errorf("file output incorrect: %s", output)
		}
	})

	t.Run("structured logging", func(t *testing.T) {
		var buf bytes.Buffer
		h := NewHandler(&buf, &Options{Level: slog.LevelInfo, UseColors: false})
		logger := slog.New(h).WithGroup("http").With("method", "GET")
		logger.Info("request completed", "path", "/api/users", "status", 200, "duration_ms", 45)

		output := buf.String()
		want := []string{`http.method="GET"`, `http.path="/api/users"`, `http.status=200`, "http.duration_ms=45"}
		for _, w := range want {
			if !strings.Contains(output, w) {
				t.Errorf("want %q in output: %s", w, output)
			}
		}
	})

	t.Run("error context", func(t *testing.T) {
		var buf bytes.Buffer
		h := NewHandler(&buf, &Options{Level: slog.LevelError, UseColors: false, AddSource: true})
		logger := slog.New(h).With("service", "api", "env", "production")
		logger.Error("database connection failed",
			"error", "connection timeout",
			"database", "users_db",
			"retry_count", 3)

		output := buf.String()
		want := []string{
			"ERROR", "database connection failed",
			`service="api"`, `env="production"`,
			`error="connection timeout"`, `database="users_db"`,
			"retry_count=3", "source=",
		}
		for _, w := range want {
			if !strings.Contains(output, w) {
				t.Errorf("want %q in output: %s", w, output)
			}
		}
	})
}

// BenchmarkHandle はログ出力のベンチマークです
func BenchmarkHandle(b *testing.B) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	for i := 0; b.Loop(); i++ {
		logger.Info("benchmark test", "iteration", i, "data", "some data")
		buf.Reset()
	}
}

// BenchmarkHandleConcurrent は並行ログ出力のベンチマークです
func BenchmarkHandleConcurrent(b *testing.B) {
	var buf bytes.Buffer
	handler := NewHandler(&buf, &Options{
		Level:     slog.LevelInfo,
		UseColors: false,
	})

	logger := slog.New(handler)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("benchmark test", "iteration", i, "data", "some data")
			i++
		}
	})
}

// 標準パッケージのslogのベンチマーク
func BenchmarkSlog(b *testing.B) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})

	logger := slog.New(handler)

	for i := 0; b.Loop(); i++ {
		logger.Info("benchmark test", "iteration", i, "data", "some data")
		buf.Reset()
	}
}

// 標準パッケージのslogの並行ログ出力のベンチマーク
func BenchmarkSlogConcurrent(b *testing.B) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	})

	logger := slog.New(handler)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("benchmark test", "iteration", i, "data", "some data")
			i++
		}
	})
}

// BenchmarkTimeFormatting はさまざまな時刻フォーマット方法のパフォーマンスを測定します
func BenchmarkTimeFormatting(b *testing.B) {
	testTime := time.Now()

	b.Run("DefaultFormatOptimized", func(b *testing.B) {
		buf := buffer.New()
		defer buf.Free()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			*buf = (*buf)[:0]
			formatTimeDefault(buf, testTime)
		}
	})

	b.Run("DefaultFormatAppendFormat", func(b *testing.B) {
		buf := buffer.New()
		defer buf.Free()
		format := "2006-01-02 15:04:05.000"
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			*buf = (*buf)[:0]
			*buf = testTime.AppendFormat(*buf, format)
		}
	})

	b.Run("RFC3339Optimized", func(b *testing.B) {
		buf := buffer.New()
		defer buf.Free()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			*buf = (*buf)[:0]
			formatTimeRFC3339(buf, testTime)
		}
	})

	b.Run("CompleteLogWithDefaultFormat", func(b *testing.B) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:      slog.LevelInfo,
			UseColors:  false,
			TimeFormat: "2006-01-02 15:04:05.000",
		})
		logger := slog.New(handler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			logger.Info("test message", "key", "value")
		}
	})

	b.Run("CompleteLogWithCustomFormat", func(b *testing.B) {
		var buf bytes.Buffer
		handler := NewHandler(&buf, &Options{
			Level:      slog.LevelInfo,
			UseColors:  false,
			TimeFormat: "2006/01/02 15:04",
		})
		logger := slog.New(handler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			logger.Info("test message", "key", "value")
		}
	})
}
