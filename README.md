# golog

⚡ 高性能でカラフルな Go ログハンドラー - `log/slog` をベースにした視認性とパフォーマンスに優れたログ出力

## 📸 表示例

gologを使うと、ログがこのように表示されます：

```
[2024-01-15 10:30:45.123] [DEBUG] msg="Application initializing" module="main"
[2024-01-15 10:30:45.456] [ INFO] msg="Server started" port=8080 env="production"
[2024-01-15 10:30:46.789] [ WARN] msg="High memory usage" usage_percent=85.3
[2024-01-15 10:30:47.012] [ERROR] msg="Database connection failed" error="timeout after 30s"
```

**カラー出力有効時**（ターミナル上）：
- 🔵 DEBUG - シアン色
- 🟢 INFO - 緑色
- 🟡 WARN - 黄色
- 🔴 ERROR - 赤色

構造化ログ、属性、グループもサポート：
```
[2024-01-15 10:30:45.123] [ INFO] msg="User action" user.id=12345 user.name="alice" action="login" ip="192.168.1.1"
[2024-01-15 10:30:45.456] [ INFO] msg="API request" http.method="POST" http.path="/api/users" http.status=201 duration_ms=42
```

## ✨ 特徴

- 🎨 **カラー出力** - ログレベルごとの色分けで視認性向上
- ⚡ **高性能** - バッファプール、事前フォーマット、最適化された時刻処理
- 📝 **構造化ログ** - 属性、グループによる構造化されたログ出力
- 🔧 **柔軟な設定** - ログレベル、時刻フォーマット、属性変換など
- 🔍 **デバッグ支援** - ソースファイル・行番号の自動追加
- 🛡️ **スレッドセーフ** - 並行処理に完全対応
- 🎯 **標準準拠** - `log/slog` の Handler インターフェースを完全実装

## 📦 インストール

```bash
go get github.com/f0reth/golog
```

## 🚀 クイックスタート

### 最もシンプルな使い方

```go
package main

import (
    "log/slog"
    "os"

    "github.com/f0reth/golog"
)

func main() {
    // ハンドラーを作成
    handler := golog.NewHandler(os.Stdout, &golog.Options{
        Level:     slog.LevelInfo,
        UseColors: true,
    })

    // ロガーを作成
    logger := slog.New(handler)

    // ログを出力
    logger.Info("Hello, golog!", "version", "1.0.0")
}
```

**出力:**
```
[2024-01-15 10:30:45.123] [ INFO] msg="Hello, golog!" version="1.0.0"
```

## 📖 使い方

### 基本的なログ出力

```go
logger := slog.New(golog.NewHandler(os.Stdout, &golog.Options{
    Level:     slog.LevelDebug,
    UseColors: true,
}))

logger.Debug("詳細なデバッグ情報", "variable", someValue)
logger.Info("通常の情報", "count", 42)
logger.Warn("警告メッセージ", "threshold", 80)
logger.Error("エラー発生", "error", err)
```

### 構造化ログと属性

属性を追加して、ログに構造化されたデータを含めることができます：

```go
// 複数の属性を追加
logger.Info("ユーザーがログインしました",
    "user_id", 12345,
    "username", "alice",
    "ip", "192.168.1.1",
    "login_time", time.Now(),
)

// 出力:
// [2024-01-15 10:30:45.123] [ INFO] msg="ユーザーがログインしました" user_id=12345 username="alice" ip="192.168.1.1" login_time="2024-01-15T10:30:45.123Z"
```

### コンテキスト付きロガー

共通の属性を持つロガーを作成できます：

```go
// ユーザーIDを常に含むロガー
userLogger := logger.With("user_id", 12345, "session_id", "abc123")

// このロガーを使うと、すべてのログに user_id と session_id が含まれる
userLogger.Info("プロフィール更新")
userLogger.Info("ファイルアップロード", "filename", "avatar.jpg")

// 出力:
// [2024-01-15 10:30:45.123] [ INFO] msg="プロフィール更新" user_id=12345 session_id="abc123"
// [2024-01-15 10:30:45.456] [ INFO] msg="ファイルアップロード" user_id=12345 session_id="abc123" filename="avatar.jpg"
```

### グループ化

関連する属性をグループ化できます：

```go
// データベース関連のログをグループ化
logger.WithGroup("database").Info("クエリ実行",
    "table", "users",
    "duration_ms", 42,
    "rows_affected", 1,
)

// 出力:
// [2024-01-15 10:30:45.123] [ INFO] msg="クエリ実行" database.table="users" database.duration_ms=42 database.rows_affected=1

// ネストされたグループ
logger.WithGroup("server").WithGroup("http").Info("リクエスト受信",
    "method", "GET",
    "path", "/api/users",
    "status", 200,
)

// 出力:
// [2024-01-15 10:30:45.123] [ INFO] msg="リクエスト受信" server.http.method="GET" server.http.path="/api/users" server.http.status=200
```

### ソースコードの場所を表示

デバッグ時にソースファイルと行番号を表示できます：

```go
handler := golog.NewHandler(os.Stdout, &golog.Options{
    Level:     slog.LevelDebug,
    UseColors: true,
    AddSource: true,  // ソース情報を追加
})

logger := slog.New(handler)
logger.Debug("デバッグメッセージ")

// 出力:
// [2024-01-15 10:30:45.123] [DEBUG] msg="デバッグメッセージ" source="main.go:42"
```

### 時刻フォーマットのカスタマイズ

```go
import "time"

// RFC3339形式
handler := golog.NewHandler(os.Stdout, &golog.Options{
    TimeFormat: time.RFC3339,
})
// 出力: [2024-01-15T10:30:45Z] [ INFO] msg="test"

// 日付のみ
handler = golog.NewHandler(os.Stdout, &golog.Options{
    TimeFormat: "2006-01-02",
})
// 出力: [2024-01-15] [ INFO] msg="test"

// カスタムフォーマット
handler = golog.NewHandler(os.Stdout, &golog.Options{
    TimeFormat: "15:04:05.000",  // 時刻のみ、ミリ秒付き
})
// 出力: [10:30:45.123] [ INFO] msg="test"

// デフォルト（省略時）: "2006-01-02 15:04:05.000"
```

### 属性の変換（ReplaceAttr）

ログ出力時に属性を変換できます：

```go
handler := golog.NewHandler(os.Stdout, &golog.Options{
    Level:     slog.LevelInfo,
    UseColors: true,
    ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
        // パスワードをマスキング
        if a.Key == "password" {
            return slog.String("password", "***REDACTED***")
        }

        // 内部属性を削除
        if a.Key == "internal_id" {
            return slog.Attr{} // 空のキー = 無視
        }

        // 時刻を固定値に（テスト用）
        if a.Key == slog.TimeKey {
            return slog.Time(slog.TimeKey, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
        }

        return a
    },
})

logger := slog.New(handler)
logger.Info("ログイン試行", "username", "alice", "password", "secret123", "internal_id", "xyz")

// 出力:
// [2000-01-01 00:00:00.000] [ INFO] msg="ログイン試行" username="alice" password="***REDACTED***"
// ※ internal_id は出力されない、時刻も変更されている
```

### カスタム型のフォーマット

#### slog.LogValuer（標準インターフェース）

```go
// 機密情報のマスキング
type Password string

func (p Password) LogValue() slog.Value {
    return slog.StringValue("[REDACTED]")
}

// カスタムフォーマット
type UserID int

func (u UserID) LogValue() slog.Value {
    return slog.StringValue(fmt.Sprintf("user_%d", u))
}

logger.Info("ユーザー認証",
    "user_id", UserID(12345),
    "password", Password("secret"),
)

// 出力:
// [2024-01-15 10:30:45.123] [ INFO] msg="ユーザー認証" user_id="user_12345" password="[REDACTED]"
```

#### LogFormatter（golog独自インターフェース）

```go
type User struct {
    ID   int
    Name string
}

func (u User) FormatForLog() (string, error) {
    return fmt.Sprintf(`"%s(id:%d)"`, u.Name, u.ID), nil
}

logger.Info("ユーザー作成", "user", User{ID: 123, Name: "Alice"})

// 出力:
// [2024-01-15 10:30:45.123] [ INFO] msg="ユーザー作成" user="Alice(id:123)"
```

## ⚙️ オプション一覧

| オプション | 型 | デフォルト | 説明 |
|-----------|-----|-----------|------|
| `Level` | `slog.Leveler` | `slog.LevelInfo` | 最小ログレベル |
| `UseColors` | `bool` | `false` | カラー出力の有効化 |
| `TimeFormat` | `string` | `"2006-01-02 15:04:05.000"` | 時刻のフォーマット |
| `AddSource` | `bool` | `false` | ソースファイル・行番号の追加 |
| `ReplaceAttr` | `func([]string, slog.Attr) slog.Attr` | `nil` | 属性の変換関数 |

## 🎯 実用例

### Webサーバーのログ

```go
handler := golog.NewHandler(os.Stdout, &golog.Options{
    Level:     slog.LevelInfo,
    UseColors: true,
})

logger := slog.New(handler).WithGroup("http")

http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
    start := time.Now()

    // リクエスト処理...

    logger.Info("リクエスト完了",
        "method", r.Method,
        "path", r.URL.Path,
        "status", 200,
        "duration_ms", time.Since(start).Milliseconds(),
        "ip", r.RemoteAddr,
    )
})

// 出力:
// [2024-01-15 10:30:45.123] [ INFO] msg="リクエスト完了" http.method="GET" http.path="/api/users" http.status=200 http.duration_ms=42 http.ip="192.168.1.1"
```

### エラーハンドリング

```go
logger := slog.New(golog.NewHandler(os.Stderr, &golog.Options{
    Level:     slog.LevelError,
    UseColors: true,
    AddSource: true,
}))

if err := doSomething(); err != nil {
    logger.Error("操作に失敗しました",
        "error", err,
        "operation", "database_update",
        "retry_count", 3,
    )
}

// 出力:
// [2024-01-15 10:30:45.123] [ERROR] msg="操作に失敗しました" source="main.go:123" error="connection timeout" operation="database_update" retry_count=3
```

### 環境別の設定

```go
func setupLogger(env string) *slog.Logger {
    opts := &golog.Options{
        UseColors: true,
    }

    switch env {
    case "production":
        opts.Level = slog.LevelWarn  // 本番は WARN 以上のみ
        opts.AddSource = false
    case "development":
        opts.Level = slog.LevelDebug  // 開発は DEBUG から
        opts.AddSource = true
    default:
        opts.Level = slog.LevelInfo
    }

    handler := golog.NewHandler(os.Stdout, opts)
    return slog.New(handler)
}
```

## ⚡ パフォーマンス

gologは高性能を実現するために以下の最適化を実装しています：

### 最適化技術

1. **バッファプール** - `sync.Pool`によるバッファの再利用
2. **事前フォーマット** - `WithAttrs`で追加された属性を事前にフォーマット
3. **最適化された時刻処理** - よく使われるフォーマットは専用の高速実装
4. **ダイレクトバッファ書き込み** - 中間文字列を作らずバッファに直接書き込み
5. **ミューテックス共有** - ハンドラーのクローン間でミューテックスを共有

### ベンチマーク結果

```
BenchmarkTimeFormatting/DefaultFormatOptimized-24     30938841    39.59 ns/op    0 B/op    0 allocs/op
BenchmarkTimeFormatting/DefaultFormatAppendFormat-24  14006256    85.43 ns/op    0 B/op    0 allocs/op

→ デフォルトフォーマットで約2倍高速化
```

## 🧪 テスト

```bash
# すべてのテストを実行
go test

# カバレッジ付き
go test -cover

# 詳細表示
go test -v

# race detector
go test -race

# ベンチマーク
go test -bench=. -benchmem
```

## 📝 ログフォーマット仕様

```
[時刻] [レベル] msg="メッセージ" key1="value1" key2=value2 ...
```

- **時刻**: 設定可能なフォーマット（デフォルト: `2006-01-02 15:04:05.000`）
- **レベル**: `DEBUG`, ` INFO`, ` WARN`, `ERROR`（5文字幅で統一）
- **msg**: ログメッセージ
- **属性**: `key=value`形式、文字列値はダブルクォートで囲まれる

### キーのエスケープ

特殊文字を含むキーは自動的にエスケープされます：

```go
logger.Info("test", "my key", "value")        // "my key"="value" （スペース）
logger.Info("test", "key=name", "value")      // "key=name"="value" （イコール）
logger.Info("test", `key"name`, "value")      // "key\"name"="value" （クォート）
```

## 🤝 貢献

バグ報告や機能リクエストは、GitHubのIssueでお願いします。

## 📄 ライセンス

このプロジェクトは Go 標準ライブラリの `log/slog` の設計パターンを参考にしています。

## 🔗 関連リンク

- [Go log/slog ドキュメント](https://pkg.go.dev/log/slog)
- [slog ハンドラーガイド](https://github.com/golang/example/tree/master/slog-handler-guide)
