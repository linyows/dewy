---
title: serverコマンドでのアプリの対応
---

# {% $markdoc.frontmatter.title %} {% #overview %}

このページでは、`server`コマンドで動かすアプリケーションに必要な対応を説明します。対応していない場合に起きる問題は[よくある質問](/ja/faq#serverコマンドでアプリケーションが起動しないのはなぜですか？)にまとめています。dewyはアプリケーションを起動する前に`--port`のポートを開き、プロセスを入れ替える間もそのポートを開いたままにし、以前のプロセスをSIGTERMで停止します。そのため、アプリケーションはdewyから渡されたソケットで待ち受け、SIGTERMを受け取ったら終了する必要があります。

`assets`コマンドはプロセスを起動しません。`container`コマンドではアプリケーションがコンテナ内で自分のポートを開きます。どちらもこのページの対応は不要です。

## dewyから渡されたソケットで待ち受ける {% #inherited-sockets %}

`server`コマンドが起動したアプリケーションは、ポートを自分で開きません。dewyはすべてのポートをすでに開いており、そのソケットを3番から始まるファイルディスクリプタとしてアプリケーションに渡します。dewyが開いているポートを開こうとすると、Linuxでは「address already in use」で失敗します。引き継いだソケットで待ち受けることで、再起動の間もポートは開いたままになります。新しいプロセスは以前のプロセスと同じソケットで接続を受け付けるため、入れ替えの間に接続が拒否されることはありません。

### Goでのsockモジュールの使い方 {% #sock %}

Goのアプリケーションは、`github.com/linyows/dewy/sock`モジュールでソケットを取得できます。このモジュールは標準ライブラリだけに依存します。

```bash
go get github.com/linyows/dewy/sock
```

`net.Listen`を`sock.Listen`に置き換えます。

```go
package main

import (
    "log"
    "net/http"

    "github.com/linyows/dewy/sock"
)

func main() {
    ln, err := sock.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(http.Serve(ln, http.DefaultServeMux))
}
```

`sock.Listen`と`sock.ListenPacket`は`net.Listen`と`net.ListenPacket`と同じ引数を取ります。dewyの下では、ポートが一致する引き継いだソケットを返します。待ち受けるインターフェースは`--host`で決まるため、アドレスのホスト部分は無視します。dewyの外で動かした場合や、dewyが開いていないポートを指定した場合は自分でアドレスを開くため、開発環境でも同じコードが動きます。

このほかに、モジュールは次の関数を提供します。

- **`sock.Listeners`と`sock.PacketConns`**：引き継いだTCPまたはUDPのソケットを、`DEWY_PORT`の順にすべて返します。[マルチポート](/ja/multi-port#inherited-sockets)のサンプルのように、`--port`で指定されたポートをそのまま提供するアプリケーションに向いています。
- **`sock.HealthListener`**：[ヘルスチェック](#health-checks)で説明するヘルスチェック用ソケットを返します。ない場合はnilを返します。
- **`sock.Inherited`と`sock.Entries`**：dewyからソケットを渡されて起動したかどうかを返し、解析済みのエントリを返します。

引き継いだソケットはそれぞれ1回だけ返し、2回目の要求には`sock.ErrTaken`を返します。`DEWY_PORT`がない場合は`SERVER_STARTER_PORT`を読みます。

### 環境変数DEWY_PORT {% #dewy-port %}

dewyは渡したソケットを環境変数`DEWY_PORT`に列挙します。同じ値を`SERVER_STARTER_PORT`にも設定します。この変数名と形式はserver-starterと互換性があるため、server-starter向けに書かれたアプリケーションは変更なしで動作します。新しく書くアプリケーションは`DEWY_PORT`を読んでください。

各エントリは`<address>=<fd>`の形式で、`;`で区切られます。`<address>`は指定したポートそのままです。`--host`に`0.0.0.0`または`::`以外のアドレスを指定した場合は`host:port`の形式になり、IPv6アドレスは角括弧で囲まれます。UDPソケットのアドレスには`/udp`が付きます。エントリはポート番号の順に並び、同じ番号ではTCPがUDPより先になります。

| `--port`と`--host` | `DEWY_PORT` |
|---|---|
| `--port 8080` | `8080=3` |
| `--port 8080,8090` | `8080=3;8090=4` |
| `--port 8080 --host 127.0.0.1` | `127.0.0.1:8080=3` |
| `--port 8080 --host ::1` | `[::1]:8080=3` |
| `--port 53,53/udp` | `53=3;53/udp=4` |
| `--port 53/udp --host ::1` | `[::1]:53/udp=3` |

`=`の後の数値はポート番号ではなくファイルディスクリプタです。

### Go以外の言語 {% #other-languages %}

Go以外の言語のアプリケーションは、`DEWY_PORT`を読んで各ディスクリプタをソケットとして開きます。PerlのServer::Starterのようなserver-starter向けのライブラリは`SERVER_STARTER_PORT`を読むため、server-starterの下と同じように動作します。

Pythonでは、`socket.socket(fileno=fd)`でディスクリプタを開けます。アドレスファミリーとソケットの種類はディスクリプタから判定されます。

```python
import os
import socket

entries = dict(e.split("=") for e in os.environ["DEWY_PORT"].split(";"))
sock = socket.socket(fileno=int(entries["8080"]))
```

Node.jsでは、`server.listen`にディスクリプタを渡せます。

```javascript
const http = require("http");

const entries = Object.fromEntries(
  process.env.DEWY_PORT.split(";").map((e) => e.split("=")),
);
http.createServer(handler).listen({ fd: Number(entries["8080"]) });
```

これらのサンプルはアドレスでエントリを探すため、`--host`を指定した場合のキーはポート番号だけでなく`host:port`になります。

## 起動と停止 {% #lifecycle %}

新しいプロセスは、`--startup-wait`（デフォルト1秒）の間動き続けた後に以前のプロセスと入れ替わります。それより早く終了したプロセスは再起動の失敗となり、以前のプロセスがそのまま提供を続けます。設定の読み込みやデータベースへの接続などで起動の数秒後に失敗しうるアプリケーションは、`--startup-wait`をその時間より長く設定してください。

入れ替えの後、dewyは以前のプロセスにSIGTERMを送ります。アプリケーションは新しい接続の受け付けをやめ、処理中のリクエストを終えてから終了します。SIGTERMの後`--drain-time`（デフォルト30秒）が経っても終了しないプロセスは、SIGKILLで強制終了されます。dewy自身がSIGINT、SIGTERM、SIGQUITで停止する場合も、同じように動作中のプロセスへSIGTERMを送ります。

SIGTERMを受け取らずに終了したプロセスは、メトリクス`dewy.server.crashes.total`でクラッシュとして数えられ、1秒から30秒まで倍々に伸びる待ち時間の後に再び起動されます。

グレースフルシャットダウンのサンプルは[シグナルハンドリング](/ja/signal-handling)を参照してください。

## ヘルスチェック {% #health-checks %}

`--health-path`を指定すると、dewyはデプロイのたびにそのパスへHTTP GETリクエストを送り、2xxまたは3xxの応答を正常とみなします。リクエストは最も小さいTCPの`--port`に送られます。送り先は`--host`のアドレスで、`--host`を省略した場合と`0.0.0.0`の場合は`127.0.0.1`、`::`の場合は`::1`です。アプリケーションは通常のポートでこのパスを提供すればよく、ほかの対応は不要です。

[`--health-before-switch`](/ja/reference#--health-before-switch)を指定すると、dewyは新しいプロセスが以前のプロセスと入れ替わる前にチェックします。dewyは新しいプロセスごとにループバックインターフェースのTCPソケットをもう1つ渡し、環境変数`DEWY_HEALTH_PORT`に`<address>=<fd>`形式の1つのエントリ（例: `127.0.0.1:41234=5`）として示します。ディスクリプタは`DEWY_PORT`のソケットに続く番号で、`--port`がない場合は3です。dewyは起動の待ち時間の後にこのソケットへ`--health-path`のリクエストを送るため、アプリケーションはこのソケットでヘルスチェックのエンドポイントを提供する必要があります。トラフィックを受け取るプロセスの状態をチェックに反映させるため、通常のポートと同じハンドラーを使ってください。

```go
package main

import (
    "log"
    "net/http"

    "github.com/linyows/dewy/sock"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    // データベース接続など、アプリケーションが依存する状態を返す。
    // リクエストを処理できないプロセスはチェックに失敗する。
    w.WriteHeader(http.StatusOK)
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", healthHandler)

    // ヘルスソケットは--health-before-switchの指定時だけ渡される
    health, err := sock.HealthListener()
    if err != nil {
        log.Fatal(err)
    }
    if health != nil {
        go http.Serve(health, mux)
    }

    ln, err := sock.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }
    log.Fatal(http.Serve(ln, mux))
}
```

ヘルスソケットは1つのプロセスだけが持つため、応答は常に以前のプロセスではなく新しいプロセスから返ります。このソケットに対応していないアプリケーションは、`--health-before-switch`の下ではすべてのヘルスチェックに失敗します。
