<p align="right"><a href="https://github.com/linyows/dewy/blob/main/README.md">English</a> | 日本語</p>

<p align="center">
  <a href="https://dewy.linyo.ws">
    <br><br><br><br><br><br>
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://github.com/linyows/dewy/blob/main/misc/dewy-dark-bg.svg?raw=true">
      <img alt="Dewy" src="https://github.com/linyows/dewy/blob/main/misc/dewy.svg?raw=true" width="240">
    </picture>
    <br><br><br><br><br><br>
  </a>
</p>

<p align="center">
  <strong>Dewy</strong> enables declarative deployment of applications in non-Kubernetes environments.
</p>

<p align="center">
  <a href="https://github.com/linyows/dewy/actions/workflows/build.yml">
    <img alt="GitHub Workflow Status" src="https://img.shields.io/github/actions/workflow/status/linyows/dewy/build.yml?branch=main&style=for-the-badge&labelColor=000000">
  </a>
  <a href="https://github.com/linyows/dewy/releases">
    <img src="http://img.shields.io/github/release/linyows/dewy.svg?style=for-the-badge&labelColor=000000" alt="GitHub Release">
  </a>
  <a href="http://godoc.org/github.com/linyows/dewy">
    <img src="http://img.shields.io/badge/go-documentation-blue.svg?style=for-the-badge&labelColor=000000" alt="Go Documentation">
  </a>
  <a href="https://deepwiki.com/linyows/dewy">
    <img src="http://img.shields.io/badge/deepwiki-documentation-purple.svg?style=for-the-badge&labelColor=000000" alt="Deepwiki Documentation">
  </a>
</p>

Dewyは、主にGoで作られたアプリケーションを非コンテナ環境において宣言的にデプロイするソフトウェアです。
Dewyは、アプリケーションのSupervisor的な役割をし、Dewyがメインプロセスとなり、子プロセスとしてアプリケーションを起動させます。
Dewyのスケジューラーは、指定する「レジストリ」をポーリングし、セマンティックバージョニングで管理された最新のバージョンを検知すると、指定する「アーティファクト」ストアからデプロイを行います。
Dewyは、いわゆるプル型のデプロイを実現します。Dewyは、レジストリ、アーティファクトストア、キャッシュストア、通知の４つのインターフェースから構成されています。
以下はDewyのデプロイプロセスと構成を図にしたものです。

<p align="center">
  <img alt="Dewyのデプロイプロセスとアーキテクチャ" src="https://github.com/linyows/dewy/blob/main/misc/dewy-architecture.svg?raw=true" width="640"/>
</p>

主な機能
--

- 宣言的プル型デプロイメント
- グレースフルリスタート
- 選択可能なレジストリとアーティファクトストア
- デプロイ状況の通知
- オーディットログ

使いかた
--

次のServerコマンドは、registryにgithub releasesを使い、8000番ポートでサーバ起動し、ログレベルをinfoに設定し、slackに通知する例です。

```sh
$ dewy server --registry ghr://linyows/myapp \
  --notifier slack://general?title=myapp -p 8000 -l info -- /opt/myapp/current/myapp
```

レジストリと通知の指定はurlを模擬した構成になっています。urlのschemeにあたる箇所はレジストリや通知の名前です。レジストリの項目で詳しく解説します。

コマンド
--

Dewyには、ServerとAssetsコマンドがあります。
ServerはServer Application用でApplicationのプロセス管理を行い、Applicationのバージョンを最新に維持します。
Assetsはhtmlやcssやjsなど、静的ファイルのバージョンを最新に維持します。

- server
- assets

デプロイフック
--

Dewyはデプロイの前後にカスタムコマンドを実行できるフック機能をサポートしています。これらのフックは作業ディレクトリでシェル(`/bin/sh -c`)経由で実行され、全ての環境変数にアクセスできます。

### フックオプション

- `--before-deploy-hook`: デプロイ開始前にコマンドを実行
- `--after-deploy-hook`: デプロイ成功後にコマンドを実行

### 使用例

```sh
# デプロイ前にデータベースをバックアップ
$ dewy server --registry ghr://myapp/api \
  --before-deploy-hook "pg_dump mydb > /backup/$(date +%Y%m%d_%H%M%S).sql" \
  --after-deploy-hook "echo 'デプロイ完了' | mail -s 'デプロイ成功' admin@example.com" \
  -- /opt/myapp/current/myapp

# デプロイ前にサービス停止、後に再起動
$ dewy server --registry ghr://myapp/api \
  --before-deploy-hook "systemctl stop nginx" \
  --after-deploy-hook "systemctl start nginx && systemctl reload nginx" \
  -- /opt/myapp/current/myapp

# デプロイ後にデータベースマイグレーション実行
$ dewy assets --registry ghr://myapp/frontend \
  --after-deploy-hook "/opt/myapp/current/migrate-db.sh"
```

### フックの動作

- **Before Hook**: before-deploy-hookが失敗するとデプロイは中止されます
- **After Hook**: デプロイ成功後のみ実行されます。失敗してもデプロイは成功扱いになります
- **実行環境**: フックは全ての環境変数を継承し、作業ディレクトリで実行されます
- **ログ**: 全てのフック実行詳細（コマンド、stdout、stderr）がログに記録されます

> [!TIP]
> **よくある用途**
> - **データベース操作**: バックアップ、マイグレーション、スキーマ更新
> - **サービス管理**: 関連サービスの停止・開始
> - **キャッシュ管理**: キャッシュクリア、新デプロイの事前ウォームアップ
> - **通知**: 内蔵通知以外のカスタムアラート
> - **ヘルスチェック**: デプロイ成功の検証
> - **設定更新**: 動的な設定変更

インターフェース
--

Dewyにはいくつかのインターフェースがあり、それぞれ選択可能な実装を持っています。以下、各インターフェースの説明をします。（もしインターフェースで欲しい実装があればissueを作ってください）

- Registry
- Artifact
- Cache
- Notifier

Registry
--

レジストリは、アプリケーションやファイルのバージョンを管理するインターフェースです。
レジストリは、Github Releases、AWS S3、GRPCから選択できます。

#### 共通オプション

共通オプションは以下の2つです。

Option　　　　　　　　　　 | Type   | Description
---         | ---    | ---
pre-release | bool   | セマンティックバージョニングにおけるプレリリースバージョンを含める場合は `true` を設定します
artifact    | string | アーティファクトのファイル名が `name_os_arch.ext` のようなフォーマットであれば Dewy パターンマッチすることができますが、そうでない場合は明示的に指定してください

### Github Releases

Github Releasesをレジストリに使う場合は以下の設定をします。また、Github APIを利用するために必要な環境変数の設定が必要です。

```sh
# 構造
ghr://<owner-name>/<repo-name>?<options: pre-release, artifact>

# 例
$ export GITHUB_TOKEN=****.....
$ dewy --registry ghr://linyows/myapp?pre-release=true&artifact=dewy.tar ...
```

### AWS S3

AWS S3をレジストリに使う場合は以下の設定をします。
オプションとしては、regionの指定とendpointの指定があります。endpointは、S3互換サービスの場合に指定してください。
また、AWS APIを利用するために必要な環境変数の設定が必要です。

```sh
# 構造
s3://<region-name>/<bucket-name>/<path-prefix>?<options: endpoint, pre-release, artifact>

# 例
$ export AWS_ACCESS_KEY_ID=****.....
$ export AWS_SECRET_ACCESS_KEY=****.....
$ dewy --registry s3://jp-north-1/dewy/foo/bar/myapp?endpoint=https://s3.isk01.sakurastorage.jp ...
```

S3でのオブジェクトのパスは、`<prefix>/<semver>/<artifact>` の順になるようにしてください。例えば次の通り。

```sh
# <prefix>/<semver>/<artifact>
foo/bar/baz/v1.2.4-rc/dewy-testapp_linux_x86_64.tar.gz
                   /dewy-testapp_linux_arm64.tar.gz
                   /dewy-testapp_darwin_arm64.tar.gz
foo/bar/baz/v1.2.3/dewy-testapp_linux_x86_64.tar.gz
                  /dewy-testapp_linux_arm64.tar.gz
                  /dewy-testapp_darwin_arm64.tar.gz
foo/bar/baz/v1.2.2/dewy-testapp_linux_x86_64.tar.gz
                  /dewy-testapp_linux_arm64.tar.gz
                  /dewy-testapp_darwin_arm64.tar.gz
```

Dewyは、 `aws-sdk-go-v2` を使っているので regionやendpointも環境変数で指定することもできます。

```sh
$ export AWS_ENDPOINT_URL="http://localhost:9000"
```

### GRPC

GRPCをレギストリに使う場合は以下の設定をします。GRPCを使う場合、アーティファクトのURLをユーザが用意するGRPCサーバ側が決めるので、pre-releaseやartifactを指定できません。
GRPCは、インターフェースを満たすサーバを自作することができ、動的にアーティファクトのURLやレポートをコントロールしたい場合にこのレジストリを使います。

```sh
# 構造
grpc://<server-host>?<options: no-tls>

# 例
$ dewy grpc://localhost:9000?no-tls=true
```

Artifact
--

アーティファクトは、アプリケーションやファイルそのものを管理するインターフェースです。
アーティファクトの実装には、Github ReleaseとAWS S3とGoogle Cloud Storageがありますが、レジストリをGRPCに選択しなければ、自動的にレジストリと同じになります。

Cache
--

キャッシュは、現在のバージョンやアーティファクトをDewyが保持するためのインターフェースです。キャッシュの実装には、ファイルシステムとメモリとHashicorp ConsulとRedisがあります。

Notifier
--

通知は、デプロイの状態を通知するインターフェースです。通知は、Slack、メール（SMTP）から選択できます。

> [!WARNING]
> `--notify`引数は非推奨となり、将来のバージョンで削除されます。代わりに`--notifier`を使用してください。

> [!IMPORTANT]
> **エラー通知制限**: Dewyは継続的な障害時のスパム防止のため、エラー通知を自動的に制限します。3回のエラー通知後、正常に復旧するまで通知が抑制され、復旧時に通知制限は自動的にリセットされます。

### Slack

Slackを通知に使う場合は以下の設定をします。オプションには、通知に付加する `title` と そのリンクである `url` が設定できます。リポジトリ名やそのURLを設定すると良いでしょう。
また、Slack APIを利用するために必要な環境変数の設定が必要です。
[Slack Appを作成](https://api.slack.com/apps)し、 OAuth Tokenを発行して設定してください。OAuthのScopeは `channels:join` と `chat:write` が必要です。

```sh
# 構造
slack://<channel-name>?<options: title, url>

# 例
$ export SLACK_TOKEN=****.....
$ dewy --notifier slack://dewy?title=myapp&url=https://dewy.liny.ws ...
```

### Mail

Mailを通知に使う場合は以下の設定をします。SMTP設定はURLパラメータまたは環境変数で指定できます。Gmailを使用する場合は、アプリパスワードを使用する必要があります。

```sh
# 構造
mail://<smtp-host>:<port>/<recipient-mail>?<options: username, password, from, subject, tls>
smtp://<smtp-host>:<port>/<recipient-mail>?<options: username, password, from, subject, tls>

# URLパラメータを使用する例
$ dewy --notifier mail://smtp.gmail.com:587/recipient@example.com?username=sender@gmail.com&password=app-password&from=sender@gmail.com&subject=Dewy+Deployment ...

# 環境変数を使用する例
$ export MAIL_USERNAME=sender@gmail.com
$ export MAIL_PASSWORD=app-password
$ export MAIL_FROM=sender@gmail.com
$ dewy --notifier mail://smtp.gmail.com:587/recipient@example.com ...
```

#### メール設定オプション

オプション | タイプ | 説明                   | デフォルト値
---        | ---    | ---                    | ---
username   | string | SMTP認証ユーザー名     | (MAIL_USERNAME環境変数から取得)
password   | string | SMTP認証パスワード     | (MAIL_PASSWORD環境変数から取得)
from       | string | 送信者メールアドレス   | (MAIL_FROM環境変数から取得、または'username'と同じ値を使用)
to         | string | 受信者メールアドレス   | (URLパスから抽出)
subject    | string | メール件名             | "Dewy Notification"
tls        | bool   | TLS暗号化を使用        | true
host       | string | SMTPサーバーホスト名   | (URLから抽出)
port       | int    | SMTPサーバーポート番号 | 587

リリース管理
--

Dewyはローカルファイルシステム内でリリースを自動管理します：

- **リリース保存**: 各デプロイは`releases/<timestamp>/`ディレクトリに保存されます
- **現在リンク**: `current`シンボリックリンクが常に最新デプロイバージョンを指します
- **自動クリーンアップ**: 最新7リリースのみ保持され、古いリリースは自動削除されます
- **ディレクトリ構造**:
  ```
  /opt/myapp/
  ├── current -> releases/20240315T143022Z/
  ├── releases/
  │   ├── 20240315T143022Z/    # 最新
  │   ├── 20240314T091534Z/
  │   ├── 20240313T172145Z/
  │   └── ...                  # 最大7リリース
  ```

セマンティックバージョニング
--

Dewyは、セマンティックバージョニングに基づいてバージョンのアーティファクトの新しい古いを判別しています。
そのため、ソフトウェアのバージョンをセマンティックバージョニングで管理しなければなりません。

詳しくは https://semver.org/lang/ja/

```txt
# Pre release versions：
v1.2.3-rc
v1.2.3-beta.2
```

デプロイワークフロー
--

次のシーケンス図は、ポーリングからサーバー再起動までのDewyのデプロイワークフローを示しています：

```mermaid
sequenceDiagram
    participant S as Scheduler
    participant D as Dewy
    participant R as Registry
    participant A as Artifact Store
    participant C as Cache
    participant F as File System
    participant H as Hooks
    participant App as Application
    participant N as Notifier

    Note over S,N: Scheduled Deployment Cycle

    S->>D: Run() - Start deployment check
    D->>R: Current() - Get latest version
    R-->>D: {ID, Tag, ArtifactURL}

    D->>C: Read("current") - Check current version
    D->>C: List() - Get cached artifacts
    C-->>D: Cached version info

    alt Version changed or not cached
        D->>A: Download(ArtifactURL)
        A-->>D: Artifact binary data
        D->>C: Write(cacheKey, artifact)
        D->>C: Write("current", cacheKey)
        Note over D,C: Cache: v1.2.3--app_linux_amd64.tar.gz
    else Version unchanged
        Note over D: Skip deployment - already current
    end

    D->>N: Send("Ready for v1.2.3")

    Note over D,App: Deployment Process

    D->>H: execHook(BeforeDeployHook)
    H-->>D: Success/Failure

    alt Before hook failed
        D->>N: SendError("Before hook failed")
        Note over D: Abort deployment
    else Before hook succeeded
        D->>F: ExtractArchive(cache → releases/timestamp/)
        D->>F: Remove old symlink
        D->>F: Symlink(releases/timestamp/ → current)

        alt Server Application
            D->>App: Start/Restart server process
            App-->>D: Process started
            D->>N: Send("Server restarted for v1.2.3")
        end

        D->>H: execHook(AfterDeployHook)
        H-->>D: Success/Failure (logged only)

        D->>R: Report({ID, Tag}) - Audit log
        D->>F: keepReleases() - Clean old releases

        Note over D,N: Success - Reset error count
        D->>N: ResetErrorCount()
    end

    Note over S,N: Cycle repeats every interval (default: 10s)
```

### 主なワークフローポイント

- **ポーリング**: Dewyは設定可能な間隔でレジストリを継続的にポーリングします
- **バージョン検出**: セマンティックバージョニングを使用して新しいリリースを検出します
- **キャッシュ**: ダウンロードはキャッシュされ、重複ダウンロードを回避します
- **アトミックデプロイ**: 古いシンボリックリンクを削除し、新しいものをアトミックに作成します
- **フック統合**: Before/afterフックがデプロイを中止またはカスタマイズできます
- **エラーハンドリング**: 失敗したデプロイはエラー通知をトリガーします（制限付き）
- **監査証跡**: 成功したデプロイはレジストリに報告されます

シグナルハンドリング
--

Dewyはプロセス管理のための各種システムシグナルに対応しています：

- **SIGHUP**: 無視（Dewyは動作を継続）
- **SIGUSR1**: 手動サーバー再起動をトリガー
- **SIGINT, SIGTERM, SIGQUIT**: グレースフルシャットダウンを開始
- **内部SIGHUP**: サーバー再起動に内部的に使用

### 手動サーバー再起動

Dewyを再起動せずにサーバーアプリケーションのみを手動で再起動できます：

```sh
# SIGUSR1を送信してサーバー再起動をトリガー
$ kill -USR1 <dewy-pid>

# systemdでDewyが管理されている場合
$ systemctl kill -s USR1 dewy.service
```

ステージング
--

セマンティックバージョニングには、プレリリースという考え方があります。バージョンに対してハイフンをつけてsuffixを付加したものが
プレリリースバージョンになります。ステージング環境では、registryのオプションに `pre-release=true`を追加することで、プレリリースバージョンがデプロイされるようになります。

システム要件
--

Dewyのデプロイには最小限のシステム要件があります：

### ファイルシステム要件

- **書き込み権限**: リリース管理のため作業ディレクトリへの書き込み権限が必要
- **シンボリックリンクサポート**: `current`ポインタのためファイルシステムでシンボリックリンクサポートが必要
- **一時ディレクトリ**: キャッシュストレージのためシステム一時ディレクトリへのアクセスが必要
- **ディスク容量**: 7リリース分とキャッシュのための十分な容量（通常数百MB）

### プロセス要件

- **シェルアクセス**: フック実行のため`/bin/sh`が利用可能である必要
- **ネットワークアクセス**: レジストリとアーティファクトストアへのアウトバウンド接続
- **シグナルハンドリング**: プロセスがシステムシグナルを受信・処理できること

> [!NOTE]
> DewyはGoランタイム（コンパイル済み）以外の外部依存関係がない単一バイナリとして動作します。

プロビジョニング
--

Dewy用のプロビジョニングは、ChefとPuppetがあります。Ansibleがないので誰か作ってくれると嬉しいです。

- Chef: https://github.com/linyows/dewy-cookbook
- Puppet: https://github.com/takumakume/puppet-dewy

背景
--

Goはコードを各環境に合わせたひとつのバイナリにコンパイルすることができます。
Kubernetesのようなオーケストレーターのある分散システムでは、Goで作られたアプリケーションのデプロイに困ることはないでしょう。
一方で、コンテナではない単一の物理ホストや仮想マシン環境において、Goのバイナリをどうやってデプロイするかの明確な答えはないように思います。
手元からscpやrsyncするshellを書いて使うのか、サーバ構成管理のansibleを使うのか、rubyのcapistranoを使うのか、方法は色々あります。
しかし、複数人のチームで誰がどこにデプロイしたといったオーディットログや情報共有を考えると、そのようなユースケースにマッチするツールがない気がします。

FAQ
--

質問されそうなことを次にまとめました。

- Latestバージョンをレジストリから削除するとどうなりますか？

    Dewyは削除後のLatestバージョンに変更します。リリースしたバージョンを削除したり上書きするのは望ましくありませんが、セキュリティの問題などやむを得ず削除するケースはあるかもしれません。
    
- オーディットログはどこにありますか？
    
    オーディットログはアーティファクトがホストされてるところにテキストファイルのファイル名として保存されます。現状は検索性がないです。何かいい方法が思いついたら変更するでしょう。
    オーディットとは別で通知としてOTELなどのオブザーバービリティプロダクトに送ることも必要かもしれません。
    
- 複数Dewyからのポーリングによってレジストリのレートリミットにかかるのはどう対処できますか？
    
    キャッシュコンポーネントにHashicorp Consul やredisを使うと複数Dewyでキャッシュを共有出来るため、レジストリへの総リクエスト数は減るでしょう。その際は、レジストリTTLを適切な時間に設定するのがよいです。
    なお、ポーリング間隔を長くするにはコマンドのオプションで指定できます。

作者
--

[@linyows](https://github.com/linyows)
