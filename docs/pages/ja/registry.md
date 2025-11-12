---
title: レジストリ
description: |
  レジストリは、アプリケーションやファイルのバージョン管理を担うDewyの中核コンポーネントです。
  Dewyはセマンティックバージョニングに基づいて最新版を自動検出し、継続的なデプロイメントを実現します。
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## 対応レジストリ

Dewyは以下のレジストリタイプに対応しています。

- **GitHub Releases** (`ghr://`): GitHubのリリース機能
- **AWS S3** (`s3://`): Amazon S3ストレージ
- **Google Cloud Storage** (`gs://`): Google Cloudストレージ
- **OCIレジストリ** (`img://`): OCI準拠のコンテナレジストリ（Docker Hub、GHCR、GCR、ECRなど）
- **gRPC** (`grpc://`): カスタムgRPCサーバー

## 共通オプション

すべてのレジストリで使用できる共通オプションがあります。

{% table %}
* オプション
* 型
* 説明
---
* `pre-release`
* bool
* プレリリースバージョンを含めるかどうか
---
* `artifact`
* string
* 自動選択されないアーティファクト名を明示指定
{% /table %}

## GitHub Releases

GitHubリリースをレジストリとして使用する最も一般的な方法です。

### 基本設定

```bash
# 基本形式
ghr://<owner>/<repo>

# 例
dewy server --registry ghr://linyows/myapp -- /opt/myapp/current/myapp
```

### 環境変数

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

GitHubのPersonal Access Tokenまたは、GitHub Actionsの場合は`GITHUB_TOKEN`を設定します。

### オプション付きの例

```bash
# プレリリース版も含める（ステージング環境）
dewy server --registry "ghr://linyows/myapp?pre-release=true"

# 特定のアーティファクトを指定
dewy server --registry "ghr://linyows/myapp?artifact=myapp-server.tar.gz"

# 両方のオプションを使用
dewy server --registry "ghr://linyows/myapp?pre-release=true&artifact=myapp-server.tar.gz"
```

### アーティファクトの自動選択

アーティファクト名を指定しない場合、Dewyは以下の規則で自動選択します：

1. 現在のOS（`linux`, `darwin`, `windows`）を含むファイル名
2. 現在のアーキテクチャ（`amd64`, `arm64`等）を含むファイル名
3. 最初にマッチしたアーティファクトを選択

例：Linux amd64環境では `myapp_linux_amd64.tar.gz` が自動選択されます。

{% callout type="important" %}
新しく作成されたリリースについては、CI/CDでのアーティファクトビルド時間を考慮して30分間のグレースピリオドがあります。
この間は「アーティファクトが見つからない」エラーが発生しても通知されません。
{% /callout %}

## AWS S3

S3互換ストレージをレジストリとして使用できます。

### 基本設定

```bash
# 基本形式
s3://<region>/<bucket>/<path-prefix>

# 例
dewy server --registry s3://ap-northeast-1/my-releases/myapp
```

### 環境変数

```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# オプション：エンドポイントURL（S3互換サービス用）
export AWS_ENDPOINT_URL=https://s3.isk01.sakurastorage.jp
```

### オブジェクトパス規則

S3内のオブジェクトは以下の構造で配置する必要があります：

```
<path-prefix>/<semver>/<artifact>
```

実際の例：

```
my-releases/myapp/v1.2.4/myapp_linux_amd64.tar.gz
my-releases/myapp/v1.2.4/myapp_linux_arm64.tar.gz
my-releases/myapp/v1.2.4/myapp_darwin_arm64.tar.gz
my-releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz
my-releases/myapp/v1.2.3/myapp_linux_arm64.tar.gz
my-releases/myapp/v1.2.3/myapp_darwin_arm64.tar.gz
```

### オプション付きの例

```bash
# カスタムエンドポイント（さくらのクラウド等）
dewy server --registry "s3://jp-north-1/my-bucket/myapp?endpoint=https://s3.isk01.sakurastorage.jp"

# プレリリース版を含める
dewy server --registry "s3://us-west-2/releases/myapp?pre-release=true"
```

## Google Cloud Storage

Google Cloud StorageをレジストリとSupport使用できます。

### 基本設定

```bash
# 基本形式
gs://<bucket>/<path-prefix>

# 例
dewy server --registry gs://my-releases-bucket/myapp
```

### 環境変数

```bash
# サービスアカウントキーを使用
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

または、Google Cloud SDKの認証情報やWorkload Identityを使用することも可能です。

### オブジェクトパス規則

S3と同様の構造で配置します：

```
myapp/v1.2.4/myapp_linux_amd64.tar.gz
myapp/v1.2.4/myapp_darwin_arm64.tar.gz
myapp/v1.2.3/myapp_linux_amd64.tar.gz
```

## OCIレジストリ

OCI準拠のコンテナレジストリは、`dewy container`コマンドでコンテナイメージのデプロイメントに使用できます。

### 対応レジストリ

DewyはOCI Distribution Specification準拠のすべてのレジストリをサポートしています：

- **GitHub Container Registry** (ghcr.io)
- **Docker Hub** (docker.io)
- **Google Artifact Registry** (gcr.io, us-docker.pkg.dev)
- **Amazon Elastic Container Registry** (ECR)
- **Azure Container Registry** (azurecr.io)
- **プライベート/セルフホスト型レジストリ** (Harbor、Nexusなど)

### 基本設定

```bash
# GitHub Container Registry
img://ghcr.io/<owner>/<repository>

# Docker Hub
img://docker.io/<owner>/<repository>
# または短縮形式
img://<owner>/<repository>

# Google Artifact Registry
img://gcr.io/<project-id>/<repository>

# プライベートレジストリ
img://registry.example.com/<repository>
```

### 認証

#### 環境変数

```bash
# ユーザー名とパスワードを使用
export DOCKER_USERNAME=myusername
export DOCKER_PASSWORD=mypassword

# またはDocker設定ファイルを使用（存在する場合自動的に使用されます）
# ~/.docker/config.json
```

#### Docker設定ファイル

Dewyは既存のDocker認証が設定されている場合、自動的に使用します：

```bash
# レジストリにログイン（認証情報は~/.docker/config.jsonに保存されます）
docker login ghcr.io
docker login docker.io

# Dewyは自動的にこれらの認証情報を使用します
dewy container --registry img://ghcr.io/myorg/myapp
```

### オプション付きの例

```bash
# 基本的な使用法
dewy container --registry img://ghcr.io/myorg/myapp

# プレリリースバージョンを含める
dewy container --registry "img://ghcr.io/myorg/myapp?pre-release=true"

# コンテナオプション付き
dewy container --registry img://ghcr.io/myorg/myapp \
  --container-port 8080 \
  --health-path /health
```

### レジストリ別の例

#### GitHub Container Registry (GHCR)

```bash
# パブリックイメージ
dewy container --registry img://ghcr.io/owner/app

# プライベートイメージ（認証が必要）
export DOCKER_USERNAME=github-username
export DOCKER_PASSWORD=ghp_personal_access_token
dewy container --registry img://ghcr.io/owner/private-app
```

#### Docker Hub

```bash
# 公式イメージ（libraryネームスペース）
dewy container --registry img://docker.io/library/nginx

# ユーザーイメージ
dewy container --registry img://docker.io/myuser/myapp

# 短縮形式（docker.ioはデフォルト）
dewy container --registry img://myuser/myapp
```

#### Google Artifact Registry

```bash
# gcloudで認証
gcloud auth configure-docker gcr.io

# イメージをデプロイ
dewy container --registry img://gcr.io/my-project/myapp
```

#### AWS ECR

```bash
# ECRにログイン
aws ecr get-login-password --region ap-northeast-1 | \
  docker login --username AWS --password-stdin \
  123456789.dkr.ecr.ap-northeast-1.amazonaws.com

# イメージをデプロイ
dewy container --registry img://123456789.dkr.ecr.ap-northeast-1.amazonaws.com/myapp
```

### タグとバージョン選択

Dewyはセマンティックバージョニングに基づいてコンテナイメージタグを自動的に選択します：

```bash
# レジストリ内のタグ:
# - v1.2.3
# - v1.2.2
# - v1.2.3-beta.1
# - latest

# 本番環境（安定版のみ、v1.2.3を選択）
dewy container --registry img://ghcr.io/myorg/myapp

# ステージング環境（プレリリースを含む、新しい場合はv1.2.3-beta.1を選択）
dewy container --registry "img://ghcr.io/myorg/myapp?pre-release=true"
```

### マルチアーキテクチャサポート

DewyはOCI Image Index（マニフェストリスト）を使用して、適切なアーキテクチャを自動的に選択します：

```bash
# ホストシステムに基づいてamd64、arm64、または他のアーキテクチャを自動的にプル
dewy container --registry img://ghcr.io/myorg/myapp
```

### Blue-Greenデプロイメントワークフロー

OCIレジストリを`dewy container`コマンドと使用する場合：

1. Dewyは指定された間隔でレジストリの新しいタグをポーリングします
2. セマンティックバージョニング準拠の新しいタグが自動的に検出されます
3. 新しいコンテナイメージがプルされます
4. 新しいコンテナのヘルスチェックが実行されます（設定されている場合）
5. ネットワークエイリアス経由でトラフィックが新しいコンテナに切り替えられます
6. 古いコンテナはドレインされて削除されます

```bash
# すべての機能を使用した完全な例
dewy container \
  --registry img://ghcr.io/myorg/myapp \
  --interval 300 \
  --container-port 8080 \
  --health-path /health \
  --health-timeout 30 \
  --drain-time 30 \
  --network production \
  --network-alias myapp-current \
  --log-level info
```

{% callout type="important" %}
OCIレジストリは、コンテナデプロイメント用の`dewy container`コマンドでのみ使用されます。
バイナリデプロイメントには、GitHub Releases、S3、またはGCSレジストリを使用してください。
{% /callout %}

## gRPC

カスタムgRPCサーバーをレジストリとして使用できます。

### 基本設定

```bash
# 基本形式
grpc://<server-host>

# 例
dewy server --registry grpc://registry.example.com:9000

# TLS無しの場合
dewy server --registry "grpc://localhost:9000?no-tls=true"
```

### 特徴

- gRPCサーバーがアーティファクトURLを動的に提供
- `pre-release`や`artifact`オプションは使用不可
- カスタムロジックによる柔軟な制御が可能

## セマンティックバージョニング

Dewyはセマンティックバージョニング（semver）に準拠したバージョン管理を行います。

### 対応形式

```bash
# 標準的なバージョン
v1.2.3
1.2.3

# プレリリース版
v1.2.3-rc
v1.2.3-beta.2
v1.2.3-alpha.1
```

### プレリリース版の使い分け

```bash
# 本番環境（安定版のみ）
dewy server --registry ghr://myorg/myapp

# ステージング環境（プレリリース版も含む）
dewy server --registry "ghr://myorg/myapp?pre-release=true"
```

## CI/CDパイプラインでの使用

```bash
# GitHub Actions
- name: Deploy with Dewy
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    dewy server --registry ghr://${{ github.repository }} \
      --log-level info --port 8080 -- /opt/app/current/app
```

## マルチステージデプロイメント

```bash
# ステージング環境
ENVIRONMENT=staging dewy server \
  --registry "ghr://myorg/myapp?pre-release=true" \
  --notifier "slack://staging-deploy?title=myapp-staging"

# 本番環境
ENVIRONMENT=production dewy server \
  --registry "ghr://myorg/myapp" \
  --notifier "slack://prod-deploy?title=myapp-prod"
```

## トラブルシューティング

### アーティファクトが見つからない

1. **バージョンタグの確認**: セマンティックバージョニングに準拠しているか
2. **アーティファクト名の確認**: OS/アーキテクチャが含まれているか
3. **権限の確認**: 認証情報が正しく設定されているか

### デバッグ方法

```bash
# デバッグログを有効にして詳細を確認
dewy server --registry ghr://owner/repo --log-level debug
```

レジストリは Dewy の動作の中核となる重要なコンポーネントです。用途に応じて適切なレジストリタイプを選択し、効率的なデプロイメント環境を構築してください。