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

アプリケーションのバージョン情報を取得するレジストリーのURLを指定します。GitHubリリース、DockerHub、ECRなど様々なレジストリーがサポートされています。

```bash
dewy server --registry ghr://owner/repo -- /opt/app/current/app
```

### --artifact (-a)

ダウンロードするアーティファクトの場所を指定します。S3、GitHub、HTTP/HTTPSなど複数のプロトコルに対応しています。

```bash
dewy server --artifact s3://bucket/path/to/artifact -- /opt/app/current/app
```

### --cache (-c)

アーティファクトのキャッシュ設定を指定します。ローカルファイルシステムやRedisをキャッシュストレージとして使用できます。

```bash
dewy server --cache file:///tmp/dewy-cache -- /opt/app/current/app
```

### --notifier (-n)

デプロイメント状況の通知設定を指定します。Slack、メールなどの通知チャンネルを設定できます。

```bash
dewy server --notifier slack://webhook-url -- /opt/app/current/app
```

### --port (-p)

アプリケーションサーバーが使用するポートを指定します。`server`コマンドではこのオプションは任意です。

指定した場合、server-starterが指定されたポートでアプリケーションを管理します。省略した場合、アプリケーションはポートをリッスンせずに実行されます（ジョブワーカー、メッセージキューコンシューマーなどに有用）。

```bash
# ポートあり（HTTPサーバー）
dewy server --port 9090 -- /opt/app/current/app

# ポートなし（ジョブワーカーなど）
dewy server --registry ghr://owner/repo -- /opt/worker/current/worker
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

レジストリーをチェックする間隔を秒単位で指定します。デフォルトは600秒（10分）です。

```bash
dewy server --interval 300 -- /opt/app/current/app
```

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

ヘルスチェック用のHTTPパスを指定します。指定すると、Dewyはトラフィックを切り替える前にこのエンドポイントが成功レスポンスを返すまで待機します。オプションです。

```bash
dewy container --registry img://ghcr.io/owner/app --health-path /health
```

### --health-timeout

ヘルスチェックのタイムアウトを秒単位で指定します。デフォルトは30秒です。

```bash
dewy container --registry img://ghcr.io/owner/app --health-timeout 60
```

### --drain-time

トラフィック切り替え後のドレイン時間を秒単位で指定します。古いコンテナはこの期間、実行中のリクエストを完了するために稼働し続けます。デフォルトは30秒です。

```bash
dewy container --registry img://ghcr.io/owner/app --drain-time 60
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

**禁止オプション:** `-d`, `-it`, `-i`, `-t`, `-l`, `-p` (これらはDewyの管理と競合します)

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

## レジストリーURL形式

Dewyは複数のレジストリータイプをサポートしています。それぞれ異なるURL形式を使用します。

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

レジストリーやアーティファクトへの接続に失敗した場合に返されます。ネットワーク接続や認証情報を確認してください。

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
  --cache redis://localhost:6379/0 \
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
