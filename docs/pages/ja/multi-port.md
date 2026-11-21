---
title: マルチポート対応
---

# {% $markdoc.frontmatter.title %} {% #overview %}

dewyは単一のアプリケーションで複数のTCPポートとUDPポートを同時に管理する機能を提供します。この機能により、HTTPとHTTPS、API とWebUI、本番用とデバッグ用など、異なる目的のサービスを統合的にデプロイできます。dewyはサーバー起動時にすべてのポートを一度だけ開いてアプリケーションに渡すため、再起動でアプリケーションのプロセスを入れ替える間もポートは閉じません。

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

### UDPポート

サフィックスのないポートはTCPです。Dockerのポート指定と同様に、ポートまたは範囲の末尾に`/udp`を付けるとUDPソケットを開きます。`/tcp`も指定でき、サフィックスなしと同じ意味になります。同じポート番号をTCPとUDPの両方で指定することもできます。

```bash
# TCPとUDPの53番ポートでDNS、8080番ポートでHTTPを提供
dewy server --registry ghr://myorg/dns \
  --port 53,53/udp,8080 \
  -- /opt/dns/current/dns

# UDPのポート範囲
dewy server --registry ghr://myorg/app --port 6000-6002/udp -- /opt/app/current/app
```

`/udp`サフィックスは`server`コマンドでのみ使用できます。`--health-path`は最も小さいTCPポートに対してリクエストを送るため、TCPポートが1つ以上必要です。UDPポートにはリクエストを送りません。

## dewyから渡されたソケットの使い方 {% #inherited-sockets %}

dewyは開いたソケットを環境変数`DEWY_PORT`に列挙し、Goのアプリケーションは`github.com/linyows/dewy/sock`モジュールでそれらを取得できます。環境変数の形式、モジュール、Go以外の言語での扱いは[アプリの対応](/ja/application)を参照してください。複数のポートを使うアプリケーションは、ポートの一覧を持つ代わりに`sock.Listeners`と`sock.PacketConns`で引き継いだソケットをすべて受け取れます。

```go
package main

import (
    "context"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/linyows/dewy/sock"
)

// serveUDPはpcで受け取ったデータグラムをそのまま送り返す。
func serveUDP(pc net.PacketConn) {
    buf := make([]byte, 65535)
    for {
        n, from, err := pc.ReadFrom(buf)
        if err != nil {
            return
        }
        pc.WriteTo(buf[:n], from)
    }
}

func main() {
    listeners, err := sock.Listeners()
    if err != nil {
        log.Fatal(err)
    }
    packetConns, err := sock.PacketConns()
    if err != nil {
        log.Fatal(err)
    }
    if !sock.Inherited() {
        // 開発時など、直接起動された場合
        l, err := net.Listen("tcp", ":8080")
        if err != nil {
            log.Fatal(err)
        }
        listeners = []net.Listener{l}
    }

    server := &MultiPortServer{}
    server.Start(listeners)
    for _, pc := range packetConns {
        go serveUDP(pc)
    }

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

dewyはアプリケーションのプロセスを入れ替える間もソケットを開いたままにします。入れ替え中に届いたUDPデータグラムはソケットのキューに入り、次に読み出したworkerが受け取ります。入れ替えの間は以前のworkerと新しいworkerが同じUDPソケットから読み出すため、各データグラムを受け取るのはどちらか一方だけです。
