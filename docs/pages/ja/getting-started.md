---
title: 使ってみよう
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewyを使って実際にアプリケーションをデプロイしてみましょう。この記事では、基本的な使い方から実際のデプロイメントまでを順を追って説明します。

## 前提条件

- Dewyがインストールされていること（[インストールガイド](/ja/installation)参照）
- バイナリ/アセットデプロイの場合: GoアプリケーションまたはアセットがGitHub Releases、S3、GCSで公開されていること
- コンテナデプロイの場合: DockerまたはPodmanがインストールされ起動していること
- 必要な環境変数が設定されていること（GitHubトークン、Docker認証情報など）

## 基本的な使い方

### サーバーアプリケーションのデプロイ

GitHub Releasesからサーバープリケーションを自動デプロイする例：

```bash
# 環境変数を設定
export GITHUB_TOKEN=your_github_token

# サーバーアプリケーションを起動
dewy server --registry ghr://owner/repo --port 8000 -- /opt/myapp/current/myapp
```

この例では：
- `ghr://owner/repo`: GitHub ReleasesのレジストリURL
- `--port 8000`: アプリケーションが使用するポート
- `/opt/myapp/current/myapp`: 実行するアプリケーションのパス

### 静的アセットのデプロイ

HTMLやCSS、JavaScriptファイルなどの静的ファイルをデプロイする場合：

```bash
dewy assets --registry ghr://owner/frontend-assets
```

### コンテナイメージのデプロイ

ゼロダウンタイムのBlue-Greenデプロイメントでコンテナ化されたアプリケーションをデプロイする場合：

```bash
# 環境変数を設定（プライベートレジストリの場合）
export DOCKER_USERNAME=your_username
export DOCKER_PASSWORD=your_password

# OCIレジストリからコンテナイメージをデプロイ
dewy container --registry img://ghcr.io/owner/app --container-port 8080
```

この例では：
- `img://ghcr.io/owner/app`: OCIレジストリURL（Docker Hub、GHCR、GCR、ECRなどに対応）
- `--container-port 8080`: コンテナがリッスンするポート
- ヘルスチェックパスは `--health-path /health` で指定可能（オプション）

Dewyは自動的に：
- Dockerネットワークが存在しない場合は作成します（デフォルト: `dewy-net`）
- ゼロダウンタイムのBlue-Greenデプロイメントを実行します
- ヘルスチェックが成功した後、新しいコンテナにトラフィックを切り替えます
- ドレイン期間後に古いコンテナを削除します

## 実際のデプロイ例

### GitHub Releasesを使った例

```bash
# GitHub Personal Access Tokenを設定
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx

# アプリケーションディレクトリを作成
sudo mkdir -p /opt/myapp
sudo chown $USER:$USER /opt/myapp
cd /opt/myapp

# Dewyを起動してサーバーアプリケーションをデプロイ
dewy server \
  --registry ghr://myorg/myapp \
  --port 8080 \
  --log-level info \
  -- /opt/myapp/current/myapp
```

### OCIレジストリを使った例

```bash
# レジストリ認証情報を設定（プライベートの場合）
export DOCKER_USERNAME=myusername
export DOCKER_PASSWORD=mypassword

# Docker/Podmanが起動していることを確認
docker info

# Blue-Greenデプロイメントでコンテナイメージをデプロイ
dewy container \
  --registry img://ghcr.io/myorg/myapp \
  --container-port 3000 \
  --health-path /health \
  --health-timeout 30 \
  --drain-time 30 \
  --log-level info
```

この例では：
- DewyはOCIレジストリをポーリングして新しいイメージバージョンをチェックします
- 新しいバージョンが検出されると、イメージをプルして新しいコンテナを起動します
- 新しいコンテナのヘルスチェックを実行します（`--health-path`が指定されている場合）
- ネットワークエイリアスを更新してトラフィックを新しいコンテナに切り替えます
- 古いコンテナはドレイン期間中も実行を続け、その後削除されます
- 新しいバージョンが公開されると、このプロセスが自動的に繰り返されます

同じネットワーク内の他のコンテナからネットワークエイリアス（デフォルト: `dewy-current`）経由でアプリケーションにアクセスできます。また、nginxやCaddyなどのリバースプロキシを通じてポートを公開することもできます。

## 次のステップ

さらに詳しく知りたい場合は、以下のドキュメントを参照してください：

- [アーキテクチャ](../architecture/)
- [Dewy CLIリファレンス](../reference/)
- [よくある質問](../faq/)
