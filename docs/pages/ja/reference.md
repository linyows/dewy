---
title: Dewy CLIリファレンス
description: Dewy CLIコマンドとオプションの完全なリファレンスガイド
---

# Dewy CLIリファレンス

このページでは、Dewy CLIのコマンド、オプション、環境変数、および使用例について詳しく説明します。

## 基本コマンド

Dewyは主に2つのコマンドを提供します：`server`と`assets`です。これらのコマンドを使用して、アプリケーションのデプロイメントと管理を行います。

### server コマンド

`dewy server`コマンドは、Dewyのメインプロセスを起動し、アプリケーションのデプロイメントと監視を行います。このコマンドがDewyの中核機能を提供します。

```bash
dewy server [オプション] -- [アプリケーションコマンド]
```

### assets コマンド

`dewy assets`コマンドは、現在のアーティファクトの詳細情報を表示するために使用されます。デプロイメントの状態確認に便利です。

```bash
dewy assets [オプション]
```

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

デプロイメント状況の通知設定を指定します。Slack、Discord、メールなどの通知チャンネルを設定できます。

```bash
dewy server --notifier slack://webhook-url -- /opt/app/current/app
```

### --port (-p)

DewyのHTTPサーバーが使用するポートを指定します。デフォルトは8080です。

```bash
dewy server --port 9090 -- /opt/app/current/app
```

### --interval (-i)

レジストリーをチェックする間隔を秒単位で指定します。デフォルトは600秒（10分）です。

```bash
dewy server --interval 300 -- /opt/app/current/app
```

### --keeptime (-k)

アーティファクトを保持する時間を秒単位で指定します。古いアーティファクトは自動的に削除されます。

```bash
dewy server --keeptime 86400 -- /opt/app/current/app
```

### --timezone (-t)

ログやスケジュールで使用するタイムゾーンを指定します。デフォルトはUTCです。

```bash
dewy server --timezone Asia/Tokyo -- /opt/app/current/app
```

### --user (-u)

アプリケーションを実行するユーザーを指定します。セキュリティ上の理由から、専用ユーザーでの実行を推奨します。

```bash
dewy server --user app-user -- /opt/app/current/app
```

### --group (-g)

アプリケーションを実行するグループを指定します。ユーザーオプションと組み合わせて使用します。

```bash
dewy server --user app-user --group app-group -- /opt/app/current/app
```

### --workdir (-w)

アプリケーションの作業ディレクトリを指定します。アプリケーションがファイルの読み書きを行う際の基準ディレクトリになります。

```bash
dewy server --workdir /opt/app/data -- /opt/app/current/app
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
```

## 環境変数

Dewyは以下の環境変数を使用して動作をカスタマイズできます。コマンドラインオプションよりも環境変数の方が優先度は低くなります。

### DEWY_REGISTRY

デフォルトのレジストリーURLを設定します。`--registry`オプションと同じ効果があります。

```bash
export DEWY_REGISTRY=ghr://owner/repo
```

### DEWY_ARTIFACT

デフォルトのアーティファクトURLを設定します。`--artifact`オプションと同じ効果があります。

```bash
export DEWY_ARTIFACT=s3://bucket/path/to/artifact
```

### DEWY_CACHE

デフォルトのキャッシュ設定を指定します。`--cache`オプションと同じ効果があります。

```bash
export DEWY_CACHE=file:///tmp/dewy-cache
```

### DEWY_NOTIFIER

デフォルトの通知設定を指定します。`--notifier`オプションと同じ効果があります。

```bash
export DEWY_NOTIFIER=slack://webhook-url
```

### DEWY_PORT

DewyのHTTPサーバーポートを設定します。`--port`オプションと同じ効果があります。

```bash
export DEWY_PORT=8080
```

### DEWY_INTERVAL

レジストリーチェック間隔を設定します。`--interval`オプションと同じ効果があります。

```bash
export DEWY_INTERVAL=600
```

### DEWY_KEEPTIME

アーティファクト保持時間を設定します。`--keeptime`オプションと同じ効果があります。

```bash
export DEWY_KEEPTIME=86400
```

### DEWY_TIMEZONE

タイムゾーンを設定します。`--timezone`オプションと同じ効果があります。

```bash
export DEWY_TIMEZONE=Asia/Tokyo
```

### DEWY_USER

実行ユーザーを設定します。`--user`オプションと同じ効果があります。

```bash
export DEWY_USER=app-user
```

### DEWY_GROUP

実行グループを設定します。`--group`オプションと同じ効果があります。

```bash
export DEWY_GROUP=app-group
```

### DEWY_WORKDIR

作業ディレクトリを設定します。`--workdir`オプションと同じ効果があります。

```bash
export DEWY_WORKDIR=/opt/app/data
```

## レジストリーURL形式

Dewyは複数のレジストリータイプをサポートしています。それぞれ異なるURL形式を使用します。

### GitHub Releases (ghr://)

GitHub Releasesからバージョン情報を取得する場合に使用します。パブリックリポジトリとプライベートリポジトリの両方に対応しています。

```bash
ghr://owner/repository
ghr://owner/repository#tag-pattern
```

### Docker Hub (dockerhub://)

Docker Hubのイメージタグからバージョン情報を取得します。コンテナ化されたアプリケーションでも使用できます。

```bash
dockerhub://namespace/repository
dockerhub://namespace/repository:tag-pattern
```

### Amazon ECR (ecr://)

Amazon Elastic Container Registryからバージョン情報を取得します。AWSの認証情報が必要です。

```bash
ecr://region/repository
ecr://account-id.dkr.ecr.region.amazonaws.com/repository
```

### Git (git://)

Gitリポジトリのタグからバージョン情報を取得します。SSH認証やHTTPS認証に対応しています。

```bash
git://github.com/owner/repository
git://gitlab.com/owner/repository
```

## 通知形式

Dewyは様々な通知チャンネルをサポートしています。デプロイメントの成功・失敗を適切な場所に通知できます。

### Slack

Slack Incoming WebhookまたはBot Tokenを使用して通知を送信します。チャンネル指定も可能です。

```bash
slack://webhook-url
slack://token@channel
```

### Discord

Discord WebhookまたはBot Tokenを使用して通知を送信します。

```bash
discord://webhook-url
discord://token@channel-id
```

### Microsoft Teams

Microsoft Teams Incoming Webhookを使用して通知を送信します。

```bash
teams://webhook-url
```

### Email (SMTP)

SMTPサーバーを通じてメール通知を送信します。認証情報とサーバー設定が必要です。

```bash
smtp://user:password@host:port/to@example.com
```

### HTTP/HTTPS

カスタムHTTPエンドポイントに通知をPOSTします。Webhook形式の通知に対応します。

```bash
http://your-webhook-endpoint
https://your-webhook-endpoint
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

### 環境変数を使用した例

環境変数を活用してコマンドラインを簡潔にする例です。Docker環境や設定管理ツールとの相性が良いアプローチです。

```bash
export DEWY_REGISTRY=ghr://mycompany/myapp
export DEWY_ARTIFACT=s3://mybucket/artifacts/
export DEWY_CACHE=file:///tmp/dewy-cache
export DEWY_NOTIFIER=slack://hooks.slack.com/services/xxx/yyy/zzz
export DEWY_PORT=8080
export DEWY_INTERVAL=300
export DEWY_TIMEZONE=Asia/Tokyo

dewy server -- /opt/app/current/myapp
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