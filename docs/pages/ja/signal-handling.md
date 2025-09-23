---
title: シグナルハンドリング
---

# {% $markdoc.frontmatter.title %} {% #overview %}

dewyはUnixシグナルを使用してプロセスの制御を行います。これにより、実行中のアプリケーションを適切に再起動したり、クリーンな終了処理を実行することができます。シグナルハンドリングは、プロダクション環境でのゼロダウンタイムデプロイメントや安全なシステム運用において重要な機能です。

## 対応シグナル一覧

dewyは以下のUnixシグナルに対応しており、それぞれ異なる動作を実行します。

### SIGHUP

SIGHUPシグナルは受信されますが、dewy自体では特別な処理は行わずに無視されます。これは、SIGHUPが子プロセス（管理対象のアプリケーション）に送信されることを想定しているためです。アプリケーション側でSIGHUPを受け取り、グレースフルリスタートや設定再読み込みなどの処理を実行することが期待されています。

### SIGUSR1

SIGUSR1シグナルを受信すると、現在実行中のサーバーアプリケーションをグレースフルに再起動します。このシグナルは、新しいバージョンのアプリケーションがデプロイされた際に手動でサーバーを再起動したい場合に使用できます。

### SIGINT

SIGINTシグナル（通常はCtrl+Cで送信）を受信すると、dewyプロセスを正常に終了します。この際、スケジューラーの停止や通知システムへの終了メッセージ送信などのクリーンアップ処理が実行されます。

### SIGTERM

SIGTERMシグナルは、プロセス管理システム（systemdなど）からの終了要求として扱われます。SIGINTと同様に、適切なクリーンアップ処理を実行してからプロセスを終了します。

### SIGQUIT

SIGQUITシグナルも強制終了シグナルとして扱われ、SIGINTやSIGTERMと同様の終了処理を実行します。

## シグナルハンドリングの実装詳細

dewyのシグナルハンドリングは`waitSigs()`関数（dewy.go:125-153）で実装されています。この関数は、指定されたシグナルを監視し続けるゴルーチンで実行されます。

```go
func (d *Dewy) waitSigs(ctx context.Context) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

    for sig := range sigCh {
        d.logger.Debug("PID received signal", slog.Int("pid", os.Getpid()), slog.String("signal", sig.String()))
        switch sig {
        case syscall.SIGHUP:
            continue
        case syscall.SIGUSR1:
            // サーバー再起動処理
        case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
            // 終了処理
        }
    }
}
```

シグナル受信時には、プロセスIDとシグナル名がログに記録され、受信したシグナルに応じて適切な処理が実行されます。

## サーバー再起動メカニズム

SIGUSR1シグナルによるサーバー再起動は、`restartServer()`関数で処理されます。この機能により、ダウンタイムなしでアプリケーションを新しいバージョンに切り替えることができます。

再起動処理では、現在のプロセスに対してSIGHUPシグナルを送信します。これにより、server-starterライブラリを使用して管理されているサーバープロセスがグレースフルリスタートを実行します。

```bash
# 手動でサーバーを再起動する例
kill -USR1 <dewy_process_id>
```

再起動が成功すると、通知システム経由で再起動完了のメッセージが送信されます。

## 終了処理

終了シグナル（SIGINT、SIGTERM、SIGQUIT）を受信すると、dewyは以下の順序でクリーンアップ処理を実行します。

まず、定期実行されているジョブスケジューラーを停止します。これにより、新しいデプロイメント処理の開始を防ぎます。次に、通知システムを通じて終了メッセージを送信し、システム管理者に対してdewyが停止することを通知します。最後に、メインの処理ループを終了してプロセスを完全に停止します。

## アプリケーション実装例

dewyと連携するアプリケーションでも、適切なシグナルハンドリングを実装することで、より安全で信頼性の高いデプロイメントが可能になります。

### HTTPサーバーアプリケーション

HTTPサーバーアプリケーションでは、Graceful shutdownを実装してリクエスト処理中の接続を適切に終了させることが重要です。

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: http.DefaultServeMux,
    }

    // シグナルチャンネルを作成
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)

        // Graceful shutdown
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := server.Shutdown(ctx); err != nil {
            log.Printf("Server shutdown error: %v", err)
        }
    }()

    log.Println("Starting server on :8080")
    if err := server.ListenAndServe(); err != http.ErrServerClosed {
        log.Printf("Server error: %v", err)
    }
}
```

このサンプルでは、SIGHUPシグナルを受信した際にGraceful shutdownを実行し、既存のリクエストを完了させてからサーバーを停止します。

### データベース接続を持つアプリケーション

データベース接続プールを使用するアプリケーションでは、終了時に接続を適切にクローズする必要があります。

```go
package main

import (
    "database/sql"
    "log"
    "os"
    "os/signal"
    "syscall"
    _ "github.com/lib/pq"
)

type App struct {
    db *sql.DB
}

func (a *App) shutdown() {
    log.Println("Closing database connections...")
    if err := a.db.Close(); err != nil {
        log.Printf("Database close error: %v", err)
    }
    log.Println("Application shutdown complete")
}

func main() {
    db, err := sql.Open("postgres", "postgresql://user:pass@localhost/db")
    if err != nil {
        log.Fatal(err)
    }

    app := &App{db: db}

    // シグナルハンドリング
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)
        app.shutdown()
        os.Exit(0)
    }()

    // アプリケーションのメイン処理
    log.Println("Application started")
    select {} // 無限待機
}
```

データベース接続プールの適切なクローズにより、接続リークを防ぎ、データベースサーバーへの負荷を軽減できます。

### バックグラウンドワーカー

長時間実行される処理を行うワーカーアプリケーションでは、処理の中断と状態保存を適切に行う必要があります。

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
)

type Worker struct {
    ctx    context.Context
    cancel context.CancelFunc
}

func (w *Worker) start() {
    w.ctx, w.cancel = context.WithCancel(context.Background())

    for {
        select {
        case <-w.ctx.Done():
            log.Println("Worker stopped")
            return
        default:
            // 長時間の処理をシミュレート
            log.Println("Processing...")
            time.Sleep(5 * time.Second)
        }
    }
}

func (w *Worker) stop() {
    log.Println("Stopping worker...")
    w.cancel()
}

func main() {
    worker := &Worker{}

    // シグナルハンドリング
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)
        worker.stop()
    }()

    log.Println("Worker started")
    worker.start()
}
```

このパターンでは、context.Contextを使用して処理の中断を制御し、シグナル受信時に安全に処理を停止できます。

### WebSocketサーバー

WebSocket接続を管理するアプリケーションでは、アクティブな接続を適切に切断する処理が必要です。

```go
package main

import (
    "log"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"

    "github.com/gorilla/websocket"
)

type WebSocketServer struct {
    clients map[*websocket.Conn]bool
    mu      sync.RWMutex
}

func (ws *WebSocketServer) addClient(conn *websocket.Conn) {
    ws.mu.Lock()
    defer ws.mu.Unlock()
    ws.clients[conn] = true
}

func (ws *WebSocketServer) removeClient(conn *websocket.Conn) {
    ws.mu.Lock()
    defer ws.mu.Unlock()
    delete(ws.clients, conn)
    conn.Close()
}

func (ws *WebSocketServer) closeAllConnections() {
    ws.mu.Lock()
    defer ws.mu.Unlock()

    log.Printf("Closing %d WebSocket connections", len(ws.clients))
    for conn := range ws.clients {
        conn.WriteMessage(websocket.CloseMessage, []byte("Server shutting down"))
        conn.Close()
    }
    ws.clients = make(map[*websocket.Conn]bool)
}

func main() {
    server := &WebSocketServer{
        clients: make(map[*websocket.Conn]bool),
    }

    upgrader := websocket.Upgrader{}

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            return
        }
        server.addClient(conn)
        defer server.removeClient(conn)

        // WebSocket処理
        for {
            _, _, err := conn.ReadMessage()
            if err != nil {
                break
            }
        }
    })

    // シグナルハンドリング
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)
        server.closeAllConnections()
        os.Exit(0)
    }()

    log.Println("WebSocket server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

WebSocket接続の適切な切断により、クライアント側でのタイムアウトエラーを防ぎ、ユーザー体験を向上させることができます。

## 実践的な使用例

プロダクション環境でのシグナル送信は、通常はプロセス管理システムやスクリプトを通じて行われます。

### systemdとの統合

systemdを使用してdewyを管理する場合、サービスファイルで適切な設定を行うことで、システムレベルでのシグナル送信が可能になります。

```systemd
[Unit]
Description=Dewy Deployment Service
After=network.target

[Service]
Type=simple
User=dewy
WorkingDirectory=/opt/dewy
ExecStart=/usr/local/bin/dewy server --registry ghr://myorg/myapp --port 8080 -- /opt/myapp/current/myapp
ExecReload=/bin/kill -USR1 $MAINPID
KillSignal=SIGTERM
TimeoutStopSec=30
Restart=always

[Install]
WantedBy=multi-user.target
```

この設定により、`systemctl reload dewy`コマンドでアプリケーションの再起動が可能になります。

### モニタリングとログ出力

シグナル受信時のログ出力は、システムの動作を監視する上で重要な情報源となります。

```bash
# dewyのログを監視
journalctl -u dewy -f

# 特定のシグナル受信ログのみを表示
journalctl -u dewy | grep "received signal"
```

ログには受信したシグナルの種類、プロセスID、実行された処理の結果が記録されるため、トラブルシューティングや運用監視に活用できます。

## トラブルシューティング

シグナルハンドリングに関する一般的な問題と解決方法について説明します。

### シグナルが正しく処理されない場合

シグナルが期待通りに処理されない場合は、まずプロセスが正常に動作しているかを確認してください。

```bash
# dewyプロセスの状態確認
ps aux | grep dewy

# プロセスにシグナルを送信
kill -USR1 <process_id>

# ログでシグナル受信を確認
tail -f /var/log/dewy.log
```

プロセスがゾンビ状態になっている場合や、権限不足でシグナルを送信できない場合があります。

### 再起動が失敗する場合

SIGUSR1による再起動が失敗する場合は、アプリケーション側のシグナルハンドリング実装に問題がある可能性があります。アプリケーションのログを確認し、SIGHUPシグナルが適切に処理されているかを検証してください。

また、server-starterライブラリの設定が正しいかどうかも確認が必要です。ポートバインドの問題や、プロセスの起動パスの設定ミスが原因となることがあります。

### ログの確認方法

問題の特定には、詳細なログレベルでの出力が有効です。

```bash
# デバッグレベルでdewyを起動
dewy server --log-level debug --registry ghr://myorg/myapp --port 8080 -- /opt/myapp/current/myapp
```

デバッグレベルのログには、シグナル受信の詳細やプロセス間通信の状況が記録されるため、問題の根本原因を特定しやすくなります。
