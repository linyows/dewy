---
title: Dewy CLIリファレンス
description: Dewy CLIコマンドとオプションの完全なリファレンスガイド
---

# Dewy CLIリファレンス

このページでは、Dewy CLIのコマンド、オプション、および使用例について詳しく説明します。

## 基本コマンド

Dewyは主に3つのコマンドを提供します：`server`、`assets`、`image`です。これらのコマンドを使用して、異なる環境でアプリケーションのデプロイメントと管理を行います。

### server コマンド

`dewy server`コマンドは、Dewyのメインプロセスを起動し、バイナリアプリケーションのデプロイメントと監視を行います。このコマンドは非コンテナデプロイメントの中核機能を提供します。

```bash
dewy server [オプション] -- [アプリケーションコマンド]
```

### assets コマンド

`dewy assets`コマンドは、現在のアーティファクトの詳細情報を表示するために使用されます。デプロイメントの状態確認に便利です。

```bash
dewy assets [オプション]
```

### container コマンド

`dewy container`コマンドは、ゼロダウンタイムのローリングアップデート戦略でコンテナイメージのデプロイメントを処理します。OCIレジストリを監視して新しいイメージバージョンを検出し、自動的にデプロイします。

```bash
dewy container [オプション]
```

#### container list サブコマンド

`dewy container list`サブコマンドは、dewyが管理している現在実行中のコンテナ情報を表示します。

```bash
dewy container list
```

**出力例:**
```
UPSTREAM           DEPLOY TIME            NAME
127.0.0.1:8080     2025-01-15 10:30:00    myapp-0
127.0.0.1:8081     2025-01-15 10:30:05    myapp-1
127.0.0.1:8082     2025-01-15 10:30:10    myapp-2
```

表示内容:
- **UPSTREAM**: プロキシのバックエンドアドレス（IP:ポート）
- **DEPLOY TIME**: コンテナがデプロイされた時刻
- **NAME**: コンテナ名（アルファベット順にソート）

**注意:** このコマンドはdewy管理APIにTCP localhostポート17539（デフォルト）で接続します。複数のdewyインスタンスがポート競合で起動している場合、自動的にポート17539-17548をスキャンします。どのディレクトリからでも実行可能です。

## コマンドラインオプション

以下のオプションを使用して、Dewyの動作をカスタマイズできます。

### --registry (-r)

アプリケーションのバージョン情報を取得するレジストリのURLを指定します。GitHubリリース、DockerHub、ECRなど様々なレジストリがサポートされています。

```bash
dewy server --registry ghr://owner/repo -- /opt/app/current/app
```

### --artifact (-a)

ダウンロードするアーティファクトの場所を指定します。S3、GitHub、HTTP/HTTPSなど複数のプロトコルに対応しています。

```bash
dewy server --artifact s3://bucket/path/to/artifact -- /opt/app/current/app
```

### --cache (-c)

URLでキャッシュbackendを選択します。サポートscheme: `file://`（デフォルトのローカルファイルシステム）、`s3://<region>/<bucket>/<prefix>`（Amazon S3およびS3互換ストレージ）、`gs://<bucket>/<prefix>`（Google Cloud Storage）。S3またはGCSを使用する場合、複数のDewyインスタンスを同じbucketに向けることでartifactダウンロードを共有でき、上流registryへのトラフィックが大幅に削減できます。

URLに `?registry-ttl=<duration>` （例: `30s`、`2m`）を追加すると、上流registryのレスポンスもインスタンス間で共有されます。TTLウィンドウあたり1台だけが上流registryをpollし、残りは共有キャッシュからレスポンスを読みます。本オプションはfile backendでは無視されます。

```bash
# ローカルファイルシステム（デフォルト）
dewy server --cache file:///tmp/dewy-cache -- /opt/app/current/app

# Amazon S3
dewy server --cache s3://ap-northeast-1/dewy-cache/myapp -- /opt/app/current/app

# Google Cloud Storage
dewy server --cache gs://dewy-cache/myapp -- /opt/app/current/app

# 共有キャッシュ + registry result cache
dewy server --cache 's3://ap-northeast-1/dewy-cache/myapp?registry-ttl=30s' -- /opt/app/current/app
```

### --notifier (-n)

デプロイメント状況の通知設定を指定します。Slack、メールなどの通知チャンネルを設定できます。

```bash
dewy server --notifier slack://webhook-url -- /opt/app/current/app
```

### --port (-p)

アプリケーションサーバーが使用するポートを指定します。`server`コマンドではこのオプションは任意です。

指定した場合、Dewyはサーバー起動時に指定されたポートを開き、Dewyが終了するまで開いたままにして、継承するファイルディスクリプタとしてアプリケーションに渡します。省略した場合、アプリケーションはポートをリッスンせずに実行されます（ジョブワーカー、メッセージキューコンシューマーなどに有用）。

ポートは単一の番号（`8080`）、カンマ区切りの一覧（`8080,8081`）、範囲（`8080-8085`）で指定でき、オプションを繰り返し指定することもできます。サフィックスのないポートはTCPです。`server`コマンドでは、ポートまたは範囲の末尾に`/udp`を付けるとUDPソケットを開きます（`53/udp`、`6000-6002/udp`）。`/tcp`も指定でき、サフィックスなしと同じ意味になります。重複は除去され、ポートは番号順に並びます。同じ番号ではTCPがUDPより先になります。

`--host`を指定しない場合、各ポートはすべてのインターフェースで待ち受けます。IPv6が使えるホストでは、ソケットはIPv4とIPv6の両方の接続を受け付けます。

アプリケーションは、環境変数`DEWY_PORT`からソケットを見つけます。この変数には、ポートごとに`<address>=<fd>`のエントリが`;`区切りで並びます。`<address>`は指定したポートそのままで、`--host`に`0.0.0.0`または`::`以外のアドレスを指定した場合は`host:port`の形式になり、IPv6アドレスは角括弧で囲まれます。UDPのエントリは末尾に`/udp`が付きます。`<fd>`は3から始まるファイルディスクリプタ番号です。server-starter向けに書かれたアプリケーションのために、dewyは同じ値を`SERVER_STARTER_PORT`にも設定します。

| `--port`と`--host` | `DEWY_PORT` |
|---|---|
| `--port 8080` | `8080=3` |
| `--port 8080 --host 127.0.0.1` | `127.0.0.1:8080=3` |
| `--port 8080 --host ::1` | `[::1]:8080=3` |
| `--port 53,53/udp` | `53=3;53/udp=4` |
| `--port 53/udp --host ::1` | `[::1]:53/udp=3` |

アプリケーションは、TCPのエントリを`net.FileListener`で、UDPのエントリを`net.FilePacketConn`で開きます。サンプルコードは[マルチポート](/ja/multi-port#inherited-sockets)を参照してください。

```bash
# ポートあり（HTTPサーバー）
dewy server --port 9090 -- /opt/app/current/app

# TCPとUDP（DNSサーバー）
dewy server --registry ghr://owner/repo --port 53,53/udp -- /opt/dns/current/dns

# ポートなし（ジョブワーカーなど）
dewy server --registry ghr://owner/repo -- /opt/worker/current/worker
```

### --host

`server`コマンドが待ち受けるIPアドレスを指定します。指定したアドレスはTCPとUDPのすべての`--port`に適用されます。省略した場合は、すべてのインターフェースで待ち受けます。IPv6が使えるホストでは、IPv4とIPv6の両方の接続を受け付けます。`0.0.0.0`を指定した場合はすべてのIPv4インターフェースだけで、`::`を指定した場合はすべてのIPv6インターフェースだけで待ち受けます。

このオプションは`--port`を必要とします。IPv4またはIPv6のユニキャストアドレスを受け付けます。IPv6アドレスは角括弧を付けずに指定します（`::1`）。リンクローカルアドレスにはゾーンを付けられます（`fe80::1%eth0`）。特定のIPv6アドレスを指定した場合はIPv6だけで待ち受けます。`localhost`のようなホスト名、マルチキャストアドレス、ブロードキャストアドレスはエラーになります。`assets`コマンドと`container`コマンドではこのオプションを受け付けません。

`0.0.0.0`と`::`以外のアドレスを指定した場合、アプリケーションに渡す環境変数`DEWY_PORT`は、各リスナーを`port=fd`ではなく`host:port=fd`の形式（例: `127.0.0.1:8000=3`、`[::1]:8000=3`）で表します。この環境変数を自前で解析するアプリケーションは、この形式に対応する必要があります。`0.0.0.0`または`::`を指定した場合は`port=fd`の形式のままです。

`--health-before-switch`なしで`--health-path`を指定した場合、ヘルスチェックのリクエストは指定したアドレスに送ります。`0.0.0.0`を指定した場合と`--host`を省略した場合は`127.0.0.1`に、`::`を指定した場合は`::1`に送ります。

```bash
# ループバックインターフェースでのみ待ち受ける
dewy server --registry ghr://owner/repo --host 127.0.0.1 --port 8000 -- /opt/app/current/app

# 特定のインターフェースで待ち受ける
dewy server --registry ghr://owner/repo --host 192.168.10.11 --port 8000-8001 -- /opt/app/current/app

# IPv6のループバックアドレスでのみ待ち受ける
dewy server --registry ghr://owner/repo --host ::1 --port 8000 -- /opt/app/current/app
```

### --startup-wait

新しいプロセスが以前のプロセスと入れ替わるまでに動作し続けている必要がある時間を秒単位で指定します。`server`コマンドでのみ使用できます。デフォルトは1秒です。`0`を指定した場合もデフォルトになり、負の値はエラーになります。

再起動（デプロイ、ロールバック、`SIGUSR1`）では、dewyは以前のプロセスと並べて新しいプロセスを起動し、この時間だけ待ちます。この時間内に新しいプロセスが終了した場合、再起動は失敗し、以前のプロセスがそのままリクエストを処理し続けます。デプロイ中の再起動が失敗した場合は、`--health-path`で説明するロールバックの処理に進みます。`dewy server`が最初に起動するプロセスにも、この時間だけ動作し続けることが求められます。

起動後に設定の読み込みやデータベースへの接続を行うアプリケーションなど、初期化の数秒後に失敗する可能性があるアプリケーションでは値を大きくしてください。

```bash
dewy server --registry ghr://owner/repo --port 8000 --startup-wait 5 -- /opt/app/current/app
```

### --health-before-switch

`--health-path`へのリクエストを、切り替え後に`--port`経由で送るのではなく、新しいプロセスが以前のプロセスと入れ替わる前に送ります。`server`コマンドでのみ使用でき、`--health-path`を必要とします。

このオプションを指定すると、新しいプロセスはそれぞれ、他のプロセスと共有しないTCPソケットを1つ追加で受け取ります。このソケットはループバックインターフェース（`127.0.0.1`、IPv4が使えない場合は`::1`）の空きポートで待ち受けます。`DEWY_PORT`のソケットに続くファイルディスクリプタとして渡され、環境変数`DEWY_HEALTH_PORT`に`<address>=<fd>`形式の1つのエントリ（例: `127.0.0.1:41234=5`）として示されます。`--port`がない場合、ディスクリプタは3です。

dewyは`--startup-wait`の時間が経過した後、成功するか`--health-timeout`を使い切るまで、このソケットの`--health-path`にリクエストを送ります。新しいプロセスが以前のプロセスと入れ替わるのは、その後です。

- 成功した場合は、通常の再起動と同様に以前のプロセスにSIGTERMを送信します。切り替え後のリクエストは送りません。
- 失敗した場合（デッドロックなどで応答がない、2xxと3xx以外のステータスが返る、プロセスが終了する）、dewyは新しいプロセスを強制終了します。以前のプロセスはリクエストの処理を一度も止めません。デプロイ中であれば、そのバージョンをblockedとして記録し、`current`シンボリックリンクを直前のリリースに戻します。以前のプロセスは動作し続けているため、再起動は行いません。通知には、直前のリリースのプロセスがリクエストの処理を止めなかったことが記載されます。`SIGUSR1`による再起動が失敗した場合は、以前のプロセスが動作し続け、それ以外は何も変わりません。

`dewy server`が最初に起動するプロセスにも、同じ方法でヘルスソケットへのリクエストを送ります。失敗した場合は起動に失敗します。

アプリケーションは`DEWY_HEALTH_PORT`のソケットでヘルスチェックのエンドポイントを提供する必要があります。dewyは、このソケットに対応していないアプリケーションと応答しなくなったアプリケーションを区別できないため、このオプションを指定すると、対応していないアプリケーションのデプロイはすべて失敗します。そのため、このオプションはデフォルトでは有効になっていません。サンプルコードは[マルチポート](/ja/multi-port#health-socket)を参照してください。

リクエストは`--port`を経由しないため、このオプションを指定した場合、`--health-path`にTCPの`--port`は必要ありません。UDPポートだけで待ち受けるアプリケーションや、`--port`のないジョブワーカーも、ヘルスソケットに対応していればヘルスチェックできます。

```bash
# 切り替え前にリクエストを送るHTTPサーバー
dewy server --registry ghr://owner/repo --port 8000 \
  --health-path /health --health-before-switch -- /opt/app/current/app

# --portのないジョブワーカーをヘルスソケットでチェックする
dewy server --registry ghr://owner/repo \
  --health-path /health --health-before-switch -- /opt/worker/current/worker
```

### --verify-checksum

ダウンロードしたアーティファクトを、併せて公開されているSHA-256チェックサムファイルとどのように照合するかを選択します。`off`、`auto`、`required`のいずれかを指定します。デフォルトは`auto`です。

- `auto`: レジストリがチェックサムファイルを見つけた場合に検証し、見つからない場合は検証せずにデプロイします。
- `required`: チェックサムファイルが公開されていない場合もデプロイを失敗させます。
- `off`: チェックサムファイルを取得しません。

検証はアーティファクトをキャッシュに書き込む前に行われます。したがって一致しなかったアーティファクトは、展開されることも、同じキャッシュバックエンドを使う他のインスタンスに共有されることもありません。認識するファイルレイアウトについては[アーティファクトの検証](/ja/artifact#アーティファクトの検証)を参照してください。

コンテナイメージは対象外です。イメージはダイジェストで指定され、pull時にコンテナランタイムが検証します。

```bash
# チェックサムファイルが公開されていないリリースのデプロイを拒否する
dewy server --registry ghr://owner/repo --verify-checksum required -- /opt/app/current/app
```

### --calver

CalVer（カレンダーバージョニング）のフォーマットを指定します。設定すると、Dewyはセマンティックバージョニングの代わりにカレンダーバージョニングを使用して最新バージョンを検出します。

**フォーマット指定子:** `YYYY`, `YY`, `0Y`, `MM`, `0M`, `WW`, `0W`, `DD`, `0D`, `MICRO`

```bash
# 4セグメントのCalVerフォーマット
dewy server --registry ghr://owner/repo --calver YYYY.0M.0D.MICRO -- /opt/app/current/app

# 3セグメントのCalVerフォーマット
dewy server --registry ghr://owner/repo --calver YYYY.0M.MICRO -- /opt/app/current/app
```

詳しくは[バージョニング - カレンダーバージョニング](/ja/versioning#calver)を参照してください。

### --interval (-i)

レジストリをチェックする間隔を秒単位で指定します。デフォルトは10秒です。

```bash
dewy server --interval 300 -- /opt/app/current/app
```

### --max-backoff-interval

チェックが失敗し続けている間、ポーリング間隔を指数関数的に伸ばします。レジストリが停止している間もリクエストの頻度が変わらないままになるのを防ぎます。値は上限を秒単位で指定します。デフォルトは`0`で、この場合バックオフは無効になり、Dewyが従来から使ってきた固定間隔のままになります。

```bash
dewy server --interval 10 --max-backoff-interval 300 -- /opt/app/current/app
```

1回の失敗では何も変わりません。2回目の連続失敗から待ち時間が倍々に伸び（`interval`、2倍、4倍、...）、そこにジッターが加わります。ここで指定した上限を超えることはありません。1回でもチェックが成功すれば、設定した間隔にすぐ戻ります。

`--interval 10 --max-backoff-interval 300`を指定すると、停止し続けているレジストリへのポーリングは10秒ごとではなくなり、間隔は最大で10秒、20秒、40秒、80秒、160秒と広がって、以降は最大300秒になります。ジッターは待ち時間を短くする方向にのみ働きます。スケジューラは`--interval`の間隔で動き続けるため、実際の待ち時間は`--interval`の整数倍になります。トレードオフとして、レジストリの復旧直後に公開されたリリースの検出が、指定した上限の時間だけ遅れることがあります。

`--interval`より短い上限を指定しても効果はありません。待ち時間が1回分の間隔より短くなることはないためです。

### --verbose (-v)

詳細なログ出力を有効にします。デバッグやトラブルシューティングに有用です。

```bash
dewy server --verbose -- /opt/app/current/app
```

### --version

Dewyのバージョン情報を表示します。

```bash
dewy --version
```

### --help (-h)

使用可能なコマンドとオプションのヘルプを表示します。

```bash
dewy --help
dewy server --help
dewy container --help
```

## containerコマンドオプション

`dewy container`コマンドには、コンテナデプロイメント管理用の固有のオプションがあります。

### --port

プロキシとコンテナ間のポートマッピングを指定します。マルチポートアプリケーションの場合、複数回指定できます。

**フォーマット:**
- `--port proxy`: DockerイメージのEXPOSEディレクティブからコンテナポートを自動検出
- `--port proxy:container`: 明示的なポートマッピング

**自動検出の動作:**
- コンテナポートが指定されていない場合、DewyはDockerイメージを検査します
- 単一のEXPOSEポート → 自動的に使用
- 複数のEXPOSEポート → エラー、明示的に指定する必要があります
- EXPOSEポートなし → エラー、明示的に指定する必要があります

**例:**

```bash
# コンテナポートを自動検出（コンテナがポート8080をEXPOSE）
dewy container --registry img://ghcr.io/owner/app --port 8080

# 明示的なポートマッピング（プロキシは8080でリッスン、コンテナポート3000に転送）
dewy container --registry img://ghcr.io/owner/app --port 8080:3000

# マルチポートアプリケーション（HTTP + gRPC）
dewy container --registry img://ghcr.io/owner/app \
  --port 8080:80 \
  --port 9090:50051
```

### --health-path

ヘルスチェック用のHTTPパスを指定します。containerコマンドとserverコマンドのどちらでもオプションです。

containerコマンドでは、新しいレプリカごとにこのパスへリクエストを送り、成功レスポンスが返ってからトラフィックを切り替えます。

serverコマンドでは、管理対象プロセスの起動または再起動の後に、指定された`--port`のうち最小の番号のTCPポートで、このパスへリクエストを送ります。再起動の場合は、新しいプロセスからの応答を得るために、以前のプロセスが終了してからリクエストを送ります。失敗した場合は直前のリリースへロールバックします。このオプションはTCPの`--port`を1つ以上必要とします。UDPポートにはリクエストを送りません。ポートを持たないサーバー（ジョブワーカーなど）にはHTTPのリクエストを送れません。assetsコマンドではこのオプションを受け付けません。

このリクエストは切り替え後に送られるため、起動はしたものの応答しない新しいプロセス（デッドロックしたプロセスなど）は、リクエストが失敗してロールバックが直前のリリースを再起動するまでトラフィックを受け取ります。その間に送られたリクエストには応答がありません。[`--health-before-switch`](/ja/reference#--health-before-switch)を指定すると、新しいプロセスごとに専用のソケットで切り替え前にリクエストを送るため、新しいプロセスが失敗しても以前のプロセスがリクエストを処理し続けます。このオプションを指定した場合、`--health-path`にTCPの`--port`は必要ありません。

ステータス2xxまたは3xxのレスポンスを成功として扱います。リダイレクトは追跡しないため、3xxはそのステータス自体で判定されます。

```bash
# container: 新しいレプリカごとにリクエストを送る
dewy container --registry img://ghcr.io/owner/app --health-path /health

# server: デプロイ後にプロセスへリクエストを送り、失敗したらロールバックする
dewy server --registry ghr://owner/repo --port 8000 --health-path /health -- /opt/app/current/app
```

### --health-timeout

1回のヘルスチェック全体の制限時間を秒単位で指定します。この制限時間には、試行間の2秒の待機を含むすべての試行が含まれます。したがって値を大きくすると、1回のリクエストを長く待つのではなく、試行回数が増えます。デフォルトは30秒です。

```bash
dewy container --registry img://ghcr.io/owner/app --health-timeout 60
```

### --no-rollback

ヘルスチェックが失敗したときに、直前のリリースへ戻さず新しいリリースをそのまま維持します。serverコマンドに適用されます。

失敗したバージョンは記録され、別のバージョンが公開されるまで再デプロイされません。失敗の通知も従来どおり送られます。dewyが自動で直前のリリースへ戻すのではなく、次の対応を運用者が判断する場合に使用します。

記録が保持するのは`<タグ>--<アーティファクト>`形式のキャッシュキーです。したがって同じタグのまま内容を修正して再公開しても解除されません。別のバージョンの公開、`--health-path`なしでの動作、キャッシュストアの`blocked`エントリの削除のいずれかで解除されます。

```bash
dewy server --registry ghr://owner/repo --port 8000 --health-path /health --no-rollback -- /opt/app/current/app
```

### --drain-time

トラフィック切り替え後のドレイン時間を秒単位で指定します。`server`コマンドと`container`コマンドで使用でき、`assets`コマンドでは受け付けません。デフォルトは30秒です。

`container`コマンドでは、古いコンテナはプロキシから外された時点で SIGTERM を受け取り、実行中のリクエストを完了するための猶予として最大この時間だけ待ってから強制終了されます。

`server`コマンドでは、新しいプロセスと入れ替わった時点で以前のプロセスにSIGTERMを送信し、この時間を過ぎても動作している場合はSIGKILLで強制終了します。dewyの終了時にも同じ制限が適用されます。dewyはすべてのプロセスにSIGTERMを送信し、この時間を過ぎても動作しているプロセスを強制終了します。

```bash
dewy container --registry img://ghcr.io/owner/app --drain-time 60
dewy server --registry ghr://owner/repo --port 8000 --drain-time 60 -- /opt/app/current/app
```

### --runtime

使用するコンテナランタイムを指定します。`docker`または`podman`をサポートします。デフォルトは`docker`です。

```bash
dewy container --registry img://ghcr.io/owner/app --runtime podman
```

### --admin-port

containerコマンド用の管理APIポートを指定します。デフォルトは17539です。ポートが既に使用されている場合、Dewyは自動的にポート番号をインクリメントします。管理APIは`dewy container list`コマンドがコンテナ情報を照会するために使用されます。

```bash
# カスタム管理ポートを使用
dewy container --registry img://ghcr.io/owner/app --admin-port 20000

# デフォルトポート（17539） - 使用中の場合は自動インクリメント
dewy container --registry img://ghcr.io/owner/app
```

**注意:** `dewy container list`コマンドは自動的にポート17539-17548をスキャンして実行中のインスタンスを検出するため、特定のポート要件がない限り、通常このオプションを指定する必要はありません。

### --cmd

コンテナに渡すコマンドと引数を指定します。複数回指定可能です。コンテナイメージのデフォルトCMDを上書きします。

```bash
dewy container --registry img://ghcr.io/owner/app \
  --cmd "/bin/sh" \
  --cmd "-c" \
  --cmd "node server.js --debug"
```

### -- (セパレータ)

`--` セパレータを使用すると、docker run の追加オプションを直接渡すことができます。`--` 以降の全ての引数は docker run コマンドに渡されます。

**サポートされるオプション:** 環境変数 (`-e`)、ボリューム (`-v`)、リソース制限 (`--cpus`, `--memory`)、エントリーポイント (`--entrypoint`) など、ほとんどの docker run オプション。

**禁止オプション:** `-d`, `-i`, `-t`, `-it`, `-p`, `--label-file` (Dewyの管理と競合)、`--privileged`, `--pid`, `--cap-add`, `--security-opt`, `--device`, `--userns`, `--cgroupns` (セキュリティに影響)。また `dewy.` ラベルプレフィックスは内部のコンテナ追跡用に予約されており、`-l` / `--label` で使用できません。

**デフォルトユーザー:** `--user` または `-u` が指定されていない場合、Dewyを実行しているユーザーのUID:GIDを使用して自動的に `--user UID:GID` を追加します。これにより、コンテナが誤ってrootで実行されることを防ぎます。コンテナにroot権限が必要な場合は、明示的に `--user=0:0` を指定してください。

**カスタムコンテナ名:** `--name` を指定してコンテナのベース名をカスタマイズできます。Dewyは自動的にタイムスタンプとレプリカインデックスを付加して一意性を保証します。

```bash
# 環境変数とボリューム
dewy container --registry img://ghcr.io/owner/app -- \
  -e API_KEY=secret \
  -e DATABASE_URL=postgres://localhost/db \
  -v /data:/app/data \
  -v /config:/app/config:ro

# リソース制限とカスタムエントリーポイント
dewy container --registry img://ghcr.io/owner/app -- \
  --cpus 2 \
  --memory 1g \
  --entrypoint /custom/entrypoint

# カスタムコンテナ名（タイムスタンプとレプリカインデックスが付加されます）
dewy container --registry img://ghcr.io/owner/app --replicas 3 -- \
  --name myapp
# 結果: myapp-1234567890-0, myapp-1234567890-1, myapp-1234567890-2
```

## レジストリURL形式

Dewyは複数のレジストリタイプをサポートしています。それぞれ異なるURL形式を使用します。

### GitHub Releases

レジストリURL形式: `ghr://`

GitHub Releasesからバージョン情報を取得する場合に使用します。パブリックリポジトリとプライベートリポジトリの両方に対応しています。

```bash
ghr://owner/repository
ghr://owner/repository?pre-release=true

# CalVerを使用する場合
dewy server --registry ghr://owner/repository --calver YYYY.0M.0D.MICRO
```

**認証:**

Dewyは2つの認証方式をサポートしています：

1. **Personal Access Token（PAT）**: `GITHUB_TOKEN`または`GH_TOKEN`環境変数を設定
2. **GitHub App**（本番環境推奨）: `GITHUB_APP_ID`、`GITHUB_APP_INSTALLATION_ID`、および`GITHUB_APP_PRIVATE_KEY`または`GITHUB_APP_PRIVATE_KEY_PATH`を設定

詳細な設定方法は[レジストリのドキュメント](/ja/registry#認証)を参照してください。

### Amazon S3

レジストリURL形式: `s3://`

Amazon S3バケットからバージョン情報を取得します。AWSの認証情報が必要です。

```bash
s3://region/bucket/prefix
s3://region/bucket/prefix?pre-release=true
```

### Google Cloud Storage

レジストリURL形式: `gs://`

Google Cloud Storageバケットからバージョン情報を取得します。GCPの認証情報が必要です。

```bash
gs://bucket/prefix
gs://bucket/prefix?pre-release=true
```

### OCIレジストリ

レジストリURL形式: `img://`

OCI互換のコンテナレジストリからバージョン情報を取得します。Docker Hub、GitHub Container Registry (GHCR)、Google Container Registry (GCR)、Amazon ECRなど、すべてのOCIレジストリに対応しています。

```bash
# Docker Hub
img://namespace/repository
img://namespace/repository?pre-release=true

# GitHub Container Registry
img://ghcr.io/owner/repository

# Google Container Registry
img://gcr.io/project/repository

# Amazon ECR
img://account-id.dkr.ecr.region.amazonaws.com/repository
```

## 通知形式

Dewyは様々な通知チャンネルをサポートしています。デプロイメントの成功・失敗を適切な場所に通知できます。

### Slack

Slack Incoming WebhookまたはBot Tokenを使用して通知を送信します。チャンネル指定も可能です。

```bash
slack://webhook-url
slack://token@channel
```

### Email (SMTP)

SMTPサーバーを通じてメール通知を送信します。認証情報とサーバー設定が必要です。

```bash
smtp://user:password@host:port/to@example.com
```

## 終了コード

Dewyは以下の終了コードを使用して実行結果を示します。スクリプトやCI/CDパイプラインでの処理分岐に活用できます。

### 正常終了 (0)

コマンドが正常に完了した場合に返されます。すべての処理が期待通りに実行されました。

### 設定エラー (1)

コマンドラインオプションや設定ファイルに問題がある場合に返されます。オプションの確認や設定の見直しが必要です。

### ネットワークエラー (2)

レジストリやアーティファクトへの接続に失敗した場合に返されます。ネットワーク接続や認証情報を確認してください。

### ファイルシステムエラー (3)

ファイルの読み書きやディレクトリアクセスに失敗した場合に返されます。権限やディスク容量を確認してください。

### アプリケーションエラー (4)

起動したアプリケーションが異常終了した場合に返されます。アプリケーションのログを確認してください。

## 使用例

以下は、Dewyの一般的な使用パターンです。実際の環境に合わせて設定を調整してください。

### 基本的な使用例

最もシンプルな設定でDewyを起動する例です。GitHub Releasesを監視してアプリケーションをデプロイします。

```bash
dewy server \
  --registry ghr://owner/repo \
  --port 8080 \
  -- /opt/app/current/myapp
```

### 完全な設定例

すべての主要なオプションを指定した包括的な設定例です。本番環境での使用に適しています。

```bash
dewy server \
  --registry ghr://mycompany/myapp \
  --artifact s3://mybucket/artifacts/ \
  --cache s3://ap-northeast-1/dewy-cache/myapp \
  --notifier slack://hooks.slack.com/services/xxx/yyy/zzz \
  --port 8080 \
  --interval 300 \
  --keeptime 86400 \
  --timezone Asia/Tokyo \
  --user app-user \
  --group app-group \
  --workdir /opt/app/data \
  --verbose \
  -- /opt/app/current/myapp --config /opt/app/config/app.conf
```

### アーティファクト情報の確認例

現在のアーティファクト状態を確認する例です。デプロイメントの状況把握に使用できます。

```bash
dewy assets --registry ghr://mycompany/myapp --verbose
```

### 開発環境での使用例

開発環境で短い間隔でチェックを行う設定例です。頻繁な更新が必要な環境に適しています。

```bash
dewy server \
  --registry ghr://mycompany/myapp-dev \
  --interval 60 \
  --port 8080 \
  --verbose \
  -- /opt/app/dev/myapp --env development
```

### コンテナイメージデプロイメントの例

ローリングアップデート戦略でコンテナイメージをデプロイする例です。OCIレジストリを監視して新しいバージョンを検出します。

```bash
# プライベートレジストリの認証
docker login ghcr.io

# 基本的なヘルスチェック付きデプロイ
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080 \
  --health-path /health \
  --health-timeout 30 \
  --drain-time 30 \
  --log-level info \
  -- \
  -e DATABASE_URL=postgres://db:5432/mydb \
  -v /data:/app/data

# カスタムコマンドでの複数レプリカ
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080 \
  --replicas 3 \
  --cmd "node" \
  --cmd "server.js" \
  --cmd "--workers=4" \
  -- \
  -e NODE_ENV=production \
  --cpus 2 \
  --memory 2g

# カスタムコンテナ名とリソース制限
dewy container \
  --registry img://ghcr.io/mycompany/myapp \
  --port 8080 \
  -- \
  --name custom-app \
  -e API_KEY=secret \
  --cpus 4 \
  --memory 4g \
  --restart unless-stopped
```
