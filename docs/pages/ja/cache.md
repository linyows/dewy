---
title: キャッシュ
description: |
  キャッシュは、ダウンロード済みアーティファクトを管理し、冗長なネットワークトラフィックを回避するDewyの重要なコンポーネントです。
  複数のキャッシュストア実装から選択でき、分散環境でのキャッシュ共有も可能です。
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## 概要 {% #overview-details %}

キャッシュコンポーネントは、Dewyのデプロイプロセスにおいて以下の重要な役割を担います：

- **アーティファクトの保存**: ダウンロードしたバイナリファイルの永続化
- **バージョン管理**: 現在のバージョン情報の記録と管理
- **重複ダウンロード防止**: 同一バージョンの再ダウンロードを回避
- **高速デプロイ**: ローカルキャッシュからの即座な展開

キャッシュは、KVSインターフェースによって抽象化されており、用途に応じて異なる実装を選択できます。

## キャッシュストア実装 {% #cache-stores %}

### ファイルシステム（File）- デフォルト {% #file-cache %}

ローカルファイルシステムにアーティファクトを保存する最も基本的な実装です。

**特徴:**
- 永続的なデータ保存
- システム再起動後もデータ保持
- シンプルな設定と管理
- アーカイブ展開機能を内蔵

**サポートされるアーカイブ形式:**
- `.tar.gz` / `.tgz`
- `.tar.bz2` / `.tbz2`
- `.tar.xz` / `.txz`
- `.tar`
- `.zip`

### メモリ（Memory）{% #memory-cache %}

{% callout type="warning" title="未実装" %}
Memoryキャッシュは現在未実装です。将来のバージョンで対応予定です。
{% /callout %}

インメモリでアーティファクトを管理する高速な実装（予定）。

**想定される特徴:**
- 高速なアクセス
- 揮発性（再起動でデータ消失）
- メモリ使用量の増加

### HashiCorp Consul {% #consul-cache %}

{% callout type="warning" title="未実装" %}
Consulキャッシュは現在未実装です。将来のバージョンで対応予定です。
{% /callout %}

分散環境でのキャッシュ共有を実現する実装（予定）。

**想定される利点:**
- 複数Dewyインスタンス間でのキャッシュ共有
- レジストリへのリクエスト削減
- 分散システムでのレート制限対策

### Redis {% #redis-cache %}

{% callout type="warning" title="未実装" %}
Redisキャッシュは現在未実装です。将来のバージョンで対応予定です。
{% /callout %}

高性能な分散キャッシュシステムとの連携実装（予定）。

**想定される特徴:**
- 高速な分散キャッシュ
- TTL設定による自動expiration
- クラスター対応

## キャッシュディレクトリ設定 {% #cache-directory %}

Dewyは、以下の優先順位でキャッシュディレクトリを決定します：

### 1. DEWY_CACHEDIR 環境変数（最高優先度）

```sh
export DEWY_CACHEDIR=/var/cache/dewy
dewy server --registry ghr://owner/repo -- /opt/myapp/current/myapp
```

### 2. カレントディレクトリ + .dewy/cache（デフォルト）

```sh
# /opt/myapp/.dewy/cache が使用される
cd /opt/myapp
dewy server --registry ghr://owner/repo -- ./current/myapp
```

### 3. 一時ディレクトリ（フォールバック）

ディレクトリ作成に失敗した場合、自動的に一時ディレクトリにフォールバックします。

### systemdでの設定例

{% callout type="note" title="systemd運用のTips" %}
systemdでDewyを管理する場合は、`DEWY_CACHEDIR`で専用のキャッシュディレクトリを指定することを推奨します。
{% /callout %}

```systemd
# /etc/systemd/system/dewy.service
[Unit]
Description=Dewy Application Deployment Service
After=network.target

[Service]
Type=simple
User=dewy
Group=dewy
Environment=DEWY_CACHEDIR=/var/cache/dewy
ExecStart=/usr/local/bin/dewy server --registry ghr://myorg/myapp -- /opt/myapp/current/myapp
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

事前にディレクトリとアクセス権を設定：

```sh
sudo mkdir -p /var/cache/dewy
sudo chown dewy:dewy /var/cache/dewy
sudo chmod 755 /var/cache/dewy
```

### Docker環境での設定

```sh
# 永続ボリュームでキャッシュを保持
docker run -d \
  -e DEWY_CACHEDIR=/app/cache \
  -v /host/dewy-cache:/app/cache \
  dewy:latest server --registry ghr://owner/repo -- /opt/app/current/app
```

## キャッシュキーの仕組み {% #cache-keys %}

Dewyは、以下のキー構造でキャッシュを管理します：

### currentキー

現在実行中のアプリケーションバージョンを示すspecialキーです。

```sh
# ファイルキャッシュの場合
cat /var/cache/dewy/current
# 出力例: v1.2.3--app_linux_amd64.tar.gz
```

この値は、実際のアーティファクトファイルを参照するキャッシュキーとして使用されます。

### アーティファクトキー

バージョンタグとアーティファクト名を組み合わせた形式：

```
{version}--{artifact_name}
```

**例:**
- `v1.2.3--myapp_linux_amd64.tar.gz`
- `v2.0.0-rc.1--myapp_darwin_arm64.zip`

## パフォーマンス最適化 {% #performance %}

### レート制限対策

複数のDewyインスタンスを運用する場合、以下の戦略でレジストリへのリクエストを削減できます：

```sh
# ポーリング間隔を長くする（デフォルト: 10秒）
dewy server --registry ghr://owner/repo \
  --interval 60s -- /opt/myapp/current/myapp

# 将来的には分散キャッシュ（Consul/Redis）でキャッシュ共有
# dewy server --registry ghr://owner/repo \
#   --cache consul://localhost:8500 \
#   --interval 30s -- /opt/myapp/current/myapp
```

### ストレージ管理

```sh
# キャッシュサイズの制限（デフォルト: 64MB）
# 現在はファイルキャッシュのディレクトリサイズで判定
du -sh /var/cache/dewy
```

## 運用ガイド {% #operations %}

### トラブルシューティング

**キャッシュミスが頻発する場合:**

```sh
# キャッシュディレクトリの確認
ls -la $DEWY_CACHEDIR

# currentキーの確認
cat $DEWY_CACHEDIR/current

# 権限の確認
ls -la $DEWY_CACHEDIR
```

**権限エラーの場合:**

```sh
# ディレクトリ権限の修正
sudo chown -R dewy:dewy /var/cache/dewy
sudo chmod -R 755 /var/cache/dewy
```

**ディスク容量不足の場合:**

```sh
# キャッシュディスクtリの使用量確認
df -h /var/cache/dewy

# 古いキャッシュファイルの手動削除
find /var/cache/dewy -name "v*" -mtime +7 -delete
```

### モニタリング

**キャッシュ利用状況の確認:**

```sh
# キャッシュファイル一覧
ls -la /var/cache/dewy/

# 現在のバージョン確認
cat /var/cache/dewy/current

# ログでキャッシュアクセスを監視
journalctl -u dewy.service -f | grep -i cache
```

## 設定例とベストプラクティス {% #best-practices %}

### 本番環境での推奨設定

```sh
# systemd環境
Environment=DEWY_CACHEDIR=/var/cache/dewy

# 適切なポーリング間隔
--interval 30s

# 構造化ログでモニタリング
--log-format json
```

### 開発環境での軽量設定

```sh
# プロジェクトディレクトリでの実行
cd /path/to/myproject
dewy server --registry ghr://owner/repo \
  --interval 5s \
  --log-format text -- ./current/myapp
```

### 高可用性構成での戦略

将来の分散キャッシュ対応時の想定設定：

```sh
# 複数インスタンスでConsulキャッシュ共有（予定）
# dewy server --registry ghr://owner/repo \
#   --cache consul://consul-cluster:8500 \
#   --interval 60s -- /opt/myapp/current/myapp
```

## 関連項目 {% #related %}

- [アーキテクチャ](/ja/architecture) - Dewyの全体構成とキャッシュの位置づけ
- [レジストリ](/ja/registry) - アーティファクトの取得元設定
- [FAQ](/ja/faq) - キャッシュ関連のよくある質問
