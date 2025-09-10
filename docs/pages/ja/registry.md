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