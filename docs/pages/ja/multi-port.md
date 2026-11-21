---
title: マルチポート対応
---

# {% $markdoc.frontmatter.title %} {% #overview %}

dewyは単一のアプリケーションで複数のTCPポートを同時に管理する機能を提供します。この機能により、HTTPとHTTPS、API とWebUI、本番用とデバッグ用など、異なる目的のサービスを統合的にデプロイできます。dewyはサーバー起動時にすべてのポートを一度だけ開いてアプリケーションに渡すため、再起動でアプリケーションのプロセスを入れ替える間もポートは閉じません。

## マルチポートのユースケース

開発環境やリソース制約のある環境では、本来であれば独立したサービスとして分離すべきものを単一のアプリケーションが複数のサービスを提供する要件があります。
dewyのマルチポート機能は、これらの複雑な要件に対応するために設計されています。

### プロトコル別

REST API、gRPC、GraphQLなどの異なるプロトコルを、単一のアプリケーションで提供したい場合があります。マルチポート機能により、管理コストを抑えながら複数のプロトコルに対応できます。

```bash
# REST API、gRPC、GraphQLを単一アプリで提供
dewy server --registry ghr://myorg/webapp --port 8080,8443,8090 -- /opt/webapp/current/webapp
```

この構成により、ポート8080でREST API、ポート8443でgRPCサービス、ポート8090でGraphQLエンドポイントを同時に提供でき、開発環境でのサーバー管理を簡素化できます。

### 機能別

単一のアプリケーションでも、API、管理画面、メトリクス収集など、異なる機能を独立したポートで提供することで、各機能に対して異なるセキュリティポリシーやアクセス制御を適用できます。

```bash
# API、管理画面、メトリクス用の複数ポート
dewy server --registry ghr://myorg/webapp \
  --port 8080,8090,9090 \
  -- /opt/webapp/current/webapp
```

この例では、単一のアプリケーションがポート8080でメインAPI、ポート8090で管理画面、ポート9090でPrometheusメトリクスを提供できます。

### 開発環境

開発環境では、本番用の機能とデバッグ用の機能を同時に提供する必要があります。デバッグポートでは詳細なログ出力やプロファイリング情報を提供し、本番ポートでは通常の動作を確認できます。

```bash
# 本番用とデバッグ用の並行運用
dewy server --registry ghr://myorg/devapp \
  --port 8080,8081 \
  -- /opt/devapp/current/devapp --prod-port=8080 --debug-port=8081
```

開発者は本番環境と同じ条件でテストを行いながら、必要に応じてデバッグ情報にもアクセスできます。

## 複数ポート指定の方法

複数のポートを指定する場合は、複数のフラグを使用するか、カンマ区切りで指定できます。どちらの方法でも同じ結果が得られるため、設定スクリプトの可読性や保守性を考慮して選択してください。

```bash
# 複数フラグによる指定
dewy server --registry ghr://myorg/app -p 8080 -p 8081 -p 9090 -- /opt/app/current/app

# カンマ区切りによる指定
dewy server --registry ghr://myorg/app --port 8080,8081,9090 -- /opt/app/current/app
```

カンマ区切りの方法は、設定ファイルや環境変数での管理に適しており、複数フラグの方法は動的な設定変更に適しています。

### ポート範囲

連続する複数のポートを指定する場合は、範囲指定を使用することで設定を簡潔に記述できます。この機能は、ロードバランシングやマルチインスタンス構成で特に有用です。

```bash
# ポート範囲の指定（8080から8085まで）
dewy server --registry ghr://myorg/app --port 8080-8085 -- /opt/app/current/app
```

範囲指定では、セキュリティとリソース管理の観点から、最大100ポートまでの制限があります。より多くのポートが必要な場合は、複数の範囲指定を組み合わせてください。

## dewyから渡されたソケットの使い方 {% #inherited-sockets %}

dewyが起動したアプリケーションは、ポートを自分で開きません。dewyはサーバー起動時にすべてのポートを開いており、そのソケットを3番から始まるファイルディスクリプタとしてアプリケーションに渡します。dewyが開いているポートを開こうとすると、Linuxでは「address already in use」で失敗します。

渡したソケットは環境変数`DEWY_PORT`に列挙されます。dewyは同じ値を`SERVER_STARTER_PORT`にも設定します。この変数名と形式はserver-starterと互換性があるため、server-starter向けに書かれたアプリケーションは変更なしで動作します。新しく書くアプリケーションは`DEWY_PORT`を読んでください。

各エントリは`<address>=<fd>`の形式で、`;`で区切られます。`<address>`は指定したポートそのままです。`--host`に`0.0.0.0`または`::`以外のアドレスを指定した場合は`host:port`の形式になり、IPv6アドレスは角括弧で囲まれます。エントリはポート番号の順に並びます。

| `--port`と`--host` | `DEWY_PORT` |
|---|---|
| `--port 8080,8090` | `8080=3;8090=4` |
| `--port 8080 --host 127.0.0.1` | `127.0.0.1:8080=3` |
| `--port 8080 --host ::1` | `[::1]:8080=3` |

`=`の後の数値はポート番号ではなくファイルディスクリプタです。アプリケーションは、ディスクリプタを`os.NewFile`と`net.FileListener`で`net.Listener`に変換します。

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "strconv"
    "strings"
    "syscall"
    "time"
)

// inheritedListenersはdewyから渡された待ち受けソケットを返す。
// DEWY_PORTがない場合、つまりdewyから起動されていない場合はnilを返す。
func inheritedListeners() ([]net.Listener, error) {
    spec := os.Getenv("DEWY_PORT")
    if spec == "" {
        return nil, nil
    }

    var listeners []net.Listener
    for _, entry := range strings.Split(spec, ";") {
        // "8080=3"や"[::1]:8080=3"：アドレス、続いてディスクリプタ
        i := strings.LastIndex(entry, "=")
        if i < 0 {
            return nil, fmt.Errorf("invalid DEWY_PORT entry: %q", entry)
        }
        addr := entry[:i]
        fd, err := strconv.Atoi(entry[i+1:])
        if err != nil {
            return nil, fmt.Errorf("invalid descriptor in %q: %w", entry, err)
        }

        l, err := net.FileListener(os.NewFile(uintptr(fd), addr))
        if err != nil {
            return nil, fmt.Errorf("cannot use descriptor %d for %s: %w", fd, addr, err)
        }
        listeners = append(listeners, l)
    }
    return listeners, nil
}

func main() {
    listeners, err := inheritedListeners()
    if err != nil {
        log.Fatal(err)
    }
    if listeners == nil {
        // 開発時など、直接起動された場合
        l, err := net.Listen("tcp", ":8080")
        if err != nil {
            log.Fatal(err)
        }
        listeners = []net.Listener{l}
    }

    server := &MultiPortServer{}
    server.Start(listeners)

    // dewyは再起動後に以前のworkerへSIGTERMを送信する
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    <-sigCh

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

`MultiPortServer`はアプリケーション側で定義する型で、受け取ったリスナーごとに`http.Server`を起動し、`Shutdown`でまとめて停止します。ソケットは設定に従って渡されるため、アプリケーションが独自にポートの一覧を持つ必要はありません。`--port`を変更すれば、アプリケーションが待ち受けるソケットも変わります。
