---
title: アーティファクト
description: |
  アーティファクトは、実際のアプリケーションバイナリやファイルを管理するDewyのコンポーネントです。
  レジストリで特定されたバージョンに対応するファイルをダウンロードし、デプロイメント用に準備します。
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## アーティファクトの種類

Dewyは以下のアーティファクトタイプに対応しています。

- **GitHub Releases** (`ghr://`): GitHubリリースの添付ファイル
- **AWS S3** (`s3://`): S3オブジェクトストレージのファイル
- **Google Cloud Storage** (`gs://`): GCSオブジェクトストレージのファイル

アーティファクトの種類は自動的にレジストリのタイプと連動します。

## ファイル形式

### 対応アーカイブ形式

Dewyは以下のアーカイブ形式をサポートしています。

- **tar.gz / tgz**: 最も一般的な形式
- **tar**: 非圧縮tar
- **zip**: Windows環境でよく使用される形式

### アーカイブ構造

アーティファクトは以下のような構造で作成することを推奨します：

```
myapp_linux_amd64.tar.gz
├── myapp                 # 実行可能バイナリ
├── config/
│   └── app.conf         # 設定ファイル
├── static/
│   ├── css/
│   └── js/
└── README.md
```

## ファイル命名規則

アーティファクト名を明示的に指定しない場合、Dewyは以下のパターンでファイルを自動選択します：

```bash
# 推奨パターン
<app-name>_<os>_<arch>.<ext>

# 例
myapp_linux_amd64.tar.gz
myapp_darwin_arm64.tar.gz
myapp_windows_amd64.zip
```

### OS識別子

{% table %}
* OS
* 識別子
* 例
---
* Linux
* `linux`
* `myapp_linux_amd64.tar.gz`
---
* macOS
* `darwin`, `macos`
* `myapp_darwin_arm64.tar.gz`
---
* Windows
* `windows`, `win`
* `myapp_windows_amd64.zip`
{% /table %}

### アーキテクチャ識別子

{% table %}
* アーキテクチャ
* 識別子
* 例
---
* x86_64
* `amd64`, `x86_64`
* `myapp_linux_amd64.tar.gz`
---
* ARM64
* `arm64`, `aarch64`
* `myapp_darwin_arm64.tar.gz`
---
* ARM32
* `arm`, `armv7`
* `myapp_linux_arm.tar.gz`
{% /table %}

## GitHub Releases でのアーティファクト

基本的な構成

```bash
# レジストリURL
ghr://owner/repo

# 自動選択される例（Linux amd64環境の場合）
myapp_linux_amd64.tar.gz
```

### リリース作成例

GitHub Actionsでのリリース作成とアーティファクト添付：

```yaml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
          - os: macos-latest
            goos: darwin
            goarch: arm64
          - os: windows-latest
            goos: windows
            goarch: amd64

    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: |
          go build -o myapp
          tar -czf myapp_${{ matrix.goos }}_${{ matrix.goarch }}.tar.gz myapp
      
      - name: Upload to release
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ steps.create_release.outputs.upload_url }}
          asset_path: ./myapp_${{ matrix.goos }}_${{ matrix.goarch }}.tar.gz
          asset_name: myapp_${{ matrix.goos }}_${{ matrix.goarch }}.tar.gz
          asset_content_type: application/gzip
```

### 特定アーティファクトの指定

```bash
# 複数のアーティファクトがある場合に特定のものを指定
dewy server --registry "ghr://owner/repo?artifact=myapp-server.tar.gz"
```

## AWS S3 でのアーティファクト

ディレクトリ構造

```
s3://my-bucket/releases/myapp/
├── v1.2.3/
│   ├── myapp_linux_amd64.tar.gz
│   ├── myapp_linux_arm64.tar.gz
│   ├── myapp_darwin_arm64.tar.gz
│   └── myapp_windows_amd64.zip
├── v1.2.2/
│   ├── myapp_linux_amd64.tar.gz
│   └── myapp_darwin_arm64.tar.gz
└── v1.2.1/
    └── myapp_linux_amd64.tar.gz
```

### アップロード例

```bash
# AWS CLIを使用したアップロード
aws s3 cp myapp_linux_amd64.tar.gz \
  s3://my-bucket/releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz

# GitHub Actionsでの自動アップロード
- name: Upload to S3
  env:
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
  run: |
    aws s3 cp myapp_linux_amd64.tar.gz \
      s3://my-bucket/releases/myapp/${GITHUB_REF_NAME}/
```

## Google Cloud Storage でのアーティファクト

ディレクトリ構造

```
gs://my-releases/myapp/
├── v1.2.3/
│   ├── myapp_linux_amd64.tar.gz
│   ├── myapp_linux_arm64.tar.gz
│   └── myapp_darwin_arm64.tar.gz
└── v1.2.2/
    └── myapp_linux_amd64.tar.gz
```

### アップロード例

```bash
# gsutil を使用したアップロード
gsutil cp myapp_linux_amd64.tar.gz \
  gs://my-releases/myapp/v1.2.3/

# GitHub Actionsでの自動アップロード
- name: Upload to GCS
  uses: google-github-actions/setup-gcloud@v1
  with:
    service_account_key: ${{ secrets.GCP_SA_KEY }}
    
- name: Upload artifact
  run: |
    gsutil cp myapp_linux_amd64.tar.gz \
      gs://my-releases/myapp/${GITHUB_REF_NAME}/
```

## アーティファクトの検証

アーティファクトの整合性を確保するため、チェックサムファイルを併せて配置することを推奨します。

```bash
# SHA256チェックサムの生成
sha256sum myapp_linux_amd64.tar.gz > myapp_linux_amd64.tar.gz.sha256

# アップロード（GitHub Releasesの例）
# - myapp_linux_amd64.tar.gz
# - myapp_linux_amd64.tar.gz.sha256
```

セキュリティを重視する環境では、GPG署名を併せて提供することも可能です。

```bash
# GPG署名の生成
gpg --detach-sign --armor myapp_linux_amd64.tar.gz

# 結果
# - myapp_linux_amd64.tar.gz
# - myapp_linux_amd64.tar.gz.asc
```

## トラブルシューティング

### アーティファクトが見つからない

命名規則の確認をする。

```bash
# 正しい例
myapp_linux_amd64.tar.gz

# 認識されない例
myapp-1.2.3.tar.gz
linux-binary.tar.gz
```

ファイルの存在確認をする。

```bash
# GitHub Releasesでの確認
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/owner/repo/releases/latest"
```

### 展開エラー

アーカイブ形式の確認をする。

```bash
# ファイル形式の確認
file myapp_linux_amd64.tar.gz

# 手動展開テスト
tar -tzf myapp_linux_amd64.tar.gz
```

権限の確認をする。

```bash
# 実行権限の設定
chmod +x myapp
```

### デバッグ方法

```bash
# アーティファクトダウンロードのデバッグ
dewy server --registry ghr://owner/repo --log-level debug

# 特定アーティファクトの指定でテスト
dewy server --registry "ghr://owner/repo?artifact=specific-file.tar.gz"
```

アーティファクト管理は Dewy のデプロイメントプロセスにおいて重要な要素です。適切な命名規則と構造化されたファイル配置により、自動デプロイメントを実現できます。