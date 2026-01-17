package loggo

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
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
			result, err := formatValue(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("expected error=%v, got error=%v", tt.hasError, err)
			}
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
	result, err := formatValue(nilPtr)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
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

	done := make(chan bool, goroutines)

	for g := range goroutines {
		go func(id int) {
			for i := range iterations {
				logger.Info("concurrent test", "goroutine", id, "iteration", i)
			}
			done <- true
		}(g)
	}

	// すべてのゴルーチンが完了するまで待つ
	for range goroutines {
		<-done
	}

	// レースコンディションが無ければテスト成功
	// （-race フラグでテストすることでレースコンディションを検出可能）
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
