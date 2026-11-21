---
title: シグナルハンドリング
---

# {% $markdoc.frontmatter.title %} {% #overview %}

dewyはUnixシグナルを使用してプロセスの制御を行います。これにより、実行中のアプリケーションを適切に再起動したり、クリーンな終了処理を実行することができます。シグナルハンドリングは、プロダクション環境でのゼロダウンタイムデプロイメントや安全なシステム運用において重要な機能です。

## 対応シグナル一覧

dewyは以下のUnixシグナルに対応しており、それぞれ異なる動作を実行します。

### SIGHUP

SIGHUPシグナルは受信されますが、dewyは何もせずに無視します。dewyはSIGHUPをアプリケーションに転送せず、アプリケーションの再起動にもSIGHUPを使いません。アプリケーションを再起動する場合はSIGUSR1を送信してください。

### SIGUSR1

SIGUSR1シグナルを受信すると、`server`コマンドで管理しているアプリケーションを、待ち受けソケットを閉じずに再起動します。このシグナルは、起動時に読み込むファイルを変更した場合など、アプリケーションを手動で再起動したい場合に使用できます。`assets`モードの場合や、アプリケーションが起動していない場合は、警告（`Restart skipped`）をログに出力するだけで何もしません。

### SIGINT

SIGINTシグナル（通常はCtrl+Cで送信）を受信すると、dewyプロセスを正常に終了します。この際、スケジューラーの停止、アプリケーションの停止、通知システムへの終了メッセージ送信などのクリーンアップ処理が実行されます。

### SIGTERM

SIGTERMシグナルは、プロセス管理システム（systemdなど）からの終了要求として扱われます。SIGINTと同様に、適切なクリーンアップ処理を実行してからプロセスを終了します。

### SIGQUIT

SIGQUITシグナルも強制終了シグナルとして扱われ、SIGINTやSIGTERMと同様の終了処理を実行します。

## シグナルハンドリングの実装詳細

dewyのシグナルハンドリングはdewy.goの`waitSigs()`関数で実装されています。この関数は、指定されたシグナルを監視し続けるゴルーチンで実行されます。

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

SIGUSR1シグナルによるサーバー再起動は、`restartServer()`関数で処理されます。新しいバージョンのデプロイとロールバックも同じ再起動処理を使います。

dewyはアプリケーションをworkerプロセスとして実行します。dewyは`--port`の待ち受けソケットをサーバー起動時に一度だけ開き、dewy自身が終了するまで開いたままにします。各workerはこのソケットを継承するため、workerを入れ替える間もポートは開いたままです。入れ替え中に届いた接続は拒否されず、カーネルのacceptキューで待機します。

再起動は次の順序で進みます。

1. dewyは現在のworkerと並べて新しいworkerを起動します。両者は同じソケットで待ち受けます。
2. dewyは起動待ち時間（`--startup-wait`、デフォルト1秒）だけ待ちます。
3. 新しいworkerが現在のworkerになり、dewyは以前のworkerにSIGTERMを送信します。
4. 以前のworkerは処理中のリクエストを完了して終了します。

新しいworkerが起動待ち時間内に終了した場合、再起動は失敗します。dewyは新しいworkerを停止し、以前のworkerはそのままリクエストを処理し続け、dewyは失敗をログに記録します。デプロイ中の再起動が失敗した場合はデプロイの失敗として扱われ、ロールバックの処理に進みます。以前のworkerが動作し続けているため、ロールバックはサーバーを再起動せずに`current`シンボリックリンクを戻します。[デプロイメントワークフロー](/ja/deployment-workflow)を参照してください。

```bash
# 手動でサーバーを再起動する例
kill -USR1 <dewy_process_id>
```

再起動が成功すると、通知システム経由で再起動完了のメッセージが送信されます。

### workerの予期しない終了

現在のworkerが停止を指示されていないのに終了した場合、dewyはWARNレベルで`Worker exited unexpectedly`をログに出力し、待機時間をおいて新しいworkerを起動します。待機時間は1秒から始まり、連続して終了するたびに2倍になり、最大30秒です。30秒以上動作したworkerが終了した場合、待機時間は1秒に戻ります。他に動作しているworkerがないため、新しいworkerは起動待ち時間やヘルスチェックを経ずにすぐ現在のworkerになります。

## 終了処理

終了シグナル（SIGINT、SIGTERM、SIGQUIT）を受信すると、dewyは以下の順序でクリーンアップ処理を実行します。

1. 定期実行されているジョブスケジューラーを停止します。これにより、新しいデプロイメント処理の開始を防ぎます。
2. workerにSIGTERMを送信し、終了を最大30秒待ちます。この時間を過ぎても動作しているworkerはSIGKILLで強制終了します。その後、待ち受けソケットを閉じます。
3. 通知システムを通じて終了メッセージを送信し、システム管理者に対してdewyが停止することを通知します。
4. メインの処理ループを終了してプロセスを完全に停止します。

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

    "github.com/linyows/dewy/sock"
)

func main() {
    // dewyの下ではdewyがポート8080に開いたソケットを返し、
    // それ以外では:8080を開く。
    listener, err := sock.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }

    server := &http.Server{
        Handler: http.DefaultServeMux,
    }

    // シグナルチャンネルを作成
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

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

    log.Printf("Starting server on %s", listener.Addr())
    if err := server.Serve(listener); err != http.ErrServerClosed {
        log.Printf("Server error: %v", err)
    }
}
```

このサンプルでは、SIGTERMシグナルを受信した際にGraceful shutdownを実行し、既存のリクエストを完了させてからサーバーを停止します。dewyは再起動後に以前のworkerへ、dewyの終了時にはすべてのworkerへSIGTERMを送信します。

dewyの下では、`--port`で指定したポートはdewyがすでに開いており、その待ち受けソケットがファイルディスクリプタ3としてアプリケーションに渡されます。`github.com/linyows/dewy/sock`モジュールの`sock.Listen`は、ポートを開き直さずに、そのソケットを`net.Listener`として返します。dewyが開いているポートを開こうとすると、Linuxでは「address already in use」で失敗します。また、渡されたソケットを使うことで、再起動の間もポートが開いたままになります。`--port`を複数指定した場合、`DEWY_PORT`にはポートごとのエントリが`;`区切りで並びます。詳しくは[マルチポート](/ja/multi-port#inherited-sockets)を参照してください。

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
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

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
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

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
    "github.com/linyows/dewy/sock"
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
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        sig := <-sigCh
        log.Printf("Received signal: %s", sig)
        server.closeAllConnections()
        os.Exit(0)
    }()

    listener, err := sock.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("WebSocket server started on %s", listener.Addr())
    log.Fatal(http.Serve(listener, nil))
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

再起動は、新しいworkerが起動待ち時間（`--startup-wait`、デフォルト1秒）内に終了した場合に失敗します。dewyはエラーとともに`Restart failure`をログに出力し、以前のworkerはそのままリクエストを処理し続けます。設定の誤りや`--`以降のコマンドパスの誤りなど、新しいworkerが終了した原因をアプリケーションのログで確認してください。

再起動は成功するのに古いworkerが残る場合は、アプリケーションがSIGTERMで終了するかを確認してください。dewyは再起動のたびに以前のworkerへSIGTERMを送信します。

dewyはソケットを起動時に一度だけ開いて保持するため、ポートを開けない問題は再起動時ではなくdewyの起動時に報告されます。

### ログの確認方法

問題の特定には、詳細なログレベルでの出力が有効です。

```bash
# デバッグレベルでdewyを起動
dewy server --log-level debug --registry ghr://myorg/myapp --port 8080 -- /opt/myapp/current/myapp
```

デバッグレベルのログには、シグナル受信の詳細やプロセス間通信の状況が記録されるため、問題の根本原因を特定しやすくなります。
