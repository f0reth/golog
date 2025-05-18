# loggo

標準ライブラリの log/slog パッケージを元にしたシンプルなカスタムログハンドラーです。

## 使い方

### インストール

```bash
go get github.com/daichi2mori/loggo
```

```go
package main

import (
	"log/slog"
	"os"

	"github.com/daichi2mori/loggo"
)

func main() {
	// ハンドラーを作成
	handler := loggo.NewHandler(os.Stdout, &loggo.Options{
		Level:     slog.LevelInfo,  // INFO以上のレベルを出力
		UseColors: true,            // 色付き出力を有効化
	})

	// ロガーを作成
	logger := slog.New(handler)

	// ログを出力
	logger.Info("Application started", "version", "1.0.0")
	logger.Warn("Warning occurred", "reason", "disk space low")
	logger.Error("Error occurred", "error", "connection failed")
}
```
