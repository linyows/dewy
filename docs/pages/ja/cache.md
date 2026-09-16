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

### AWS S3 {% #s3-cache %}

Amazon S3（およびS3互換オブジェクトストレージ）をbackendとする共有キャッシュです。S3 bucketがnode間で共有される真実のソースとなり、各Dewyインスタンスはアーカイブ展開のためのローカルstagingコピーを保持します。

**特徴:**
- 多数のDewyインスタンス間でキャッシュを共有
- 上流registryへのartifactダウンロードトラフィックを大幅に削減
- registry sourceとして使用しているAWS認証情報をそのまま流用可能
- ローカルstagingにより、展開処理はfile backendと同一

**URL形式:**

```sh
# s3://<region>/<bucket>/<prefix>?<options: endpoint>
dewy server --registry ghr://owner/repo \
  --cache s3://ap-northeast-1/mybucket/myapp -- /opt/myapp/current/myapp
```

S3互換サービス向けに`endpoint`クエリパラメーターをサポートしています。AWS認証は標準のcredential chain（環境変数、shared config、IAM roleなど）を使用します。

### Google Cloud Storage {% #gcs-cache %}

Google Cloud Storageをbackendとする共有キャッシュです。S3 backendと同じハイブリッドモデル（cloudが真実のソース、ローカルstagingで展開）です。

**URL形式:**

```sh
# gs://<bucket>/<prefix>
dewy server --registry ghr://owner/repo \
  --cache gs://mybucket/myapp -- /opt/myapp/current/myapp
```

認証はGoogle Cloud標準の方式（`GOOGLE_APPLICATION_CREDENTIALS`、workload identity、ADC）に従います。

### Registry result cache {% #registry-result-cache %}

S3とGCSのcache backendは `registry-ttl=<duration>` query parameterを受け付けます。指定すると**上流registryのレスポンスそのもの**もキャッシュに保存されます。同じprefixを共有するDewyインスタンス間では排他制御がかかり、TTLウィンドウあたり1台だけが上流registryをpollするようになります。GitHub Releasesのようなrate-limit付きregistryを多数のDewyインスタンスでpollする場合に有効です。

```sh
# 30秒のfreshness windowで共有registry-result cacheを有効化
dewy server --registry ghr://owner/repo \
  --cache 's3://ap-northeast-1/mybucket/myapp?registry-ttl=30s' \
  -- /opt/myapp/current/myapp

# GCSでも同様
dewy server --registry ghr://owner/repo \
  --cache 'gs://mybucket/myapp?registry-ttl=30s' \
  -- /opt/myapp/current/myapp
```

refreshは同じprefixの `locks/` 配下に置かれるlock recordで直列化され、結果はconditional write（`If-Match` / `ifGenerationMatch`）で公開されます。上流registry障害時は最後のキャッシュ値を返し続けるため（stale-but-usable）、一時的なregistry障害でクラスタが止まりません。

backendがエラーを返してlockを取得できなかった場合、Dewyは `"failed to acquire registry refresh lock"` を出力し、上流registryを直接pollします。そのtickでは排他制御が効きませんが、デプロイは継続します。

> 運用上の注意: stale-but-usableは `Dewy.Run()` の通常のエラー経路から上流エラーを隠すため、長期障害が設定済みのnotifierに通知されません。dewyログ内の `"upstream registry failed; serving stale cache"` warningを監視してください。

conditional writeをサポートしないbackend（現状はfile backend）に `registry-ttl` を設定した場合、Dewyは起動時に `"registry-ttl set but cache backend does not support atomic writes; ignoring"` warningを出力し、registry-result cacheを有効化せずに動作を続行します。

### Artifact downloadの排他制御 {% #download-coordination %}

S3とGCSのbackendでは、artifactのdownload自体もインスタンス間で排他制御されます。設定は不要で、`registry-ttl` とも独立しています。backendがconditional writeをサポートしていれば常に有効です。

排他制御がない場合、同じタイミングでpollしたインスタンスがすべてcacheをmissし、すべてが同じartifactをdownloadします。これはインスタンスあたり最大512MBのリクエストがregistryに向かうことを意味します。

1台が `locks/` 配下のartifactごとのlockを取得してdownloadします。残りは `"Deploy deferred: a peer is downloading this artifact"` を出力してそのtickをスキップします。次のpoll時にはartifactが共有cacheに存在するため、downloadせずに通常のcache経路を通ります。スキップしたtickは失敗として扱われません。polling backoffを発動せず、notifierにも通知せず、デプロイとしてもカウントされません。

downloadしたインスタンスは、artifactのSHA-256 digestとサイズを `blobs/<cache key>.json` に記録します。共有cacheからartifactを読んだインスタンスは、そのバイト列を記録と照合します。fresh downloadが通るchecksum検証を、これらのインスタンスは経由していないためです。不一致の場合はtickを失敗させ、ローカルのコピーを削除します。次のpollで同じバイト列をディスクから読み直さないようにするためです。

この機能より前のDewyでcacheされたartifactにはdigestの記録がありません。その場合は照合せずに使用し、artifactが再度downloadされた時点で記録が作られます。読み取りに失敗した記録は同じ扱いにしません。「確実に存在しない」場合だけ照合を省略するため、壊れた記録はtickを失敗させます。

digest不一致の際に削除するのはローカルのコピーだけです。バケット上のオブジェクトはそのまま残します。1台のディスク不良で全インスタンスにre-downloadを強いるべきではなく、実際に改竄されたオブジェクトは黙って置き換えるのではなく調査されるべきだからです。

file backendは単一インスタンス向けであり、影響を受けません。downloadは従来どおり動作します。

### デプロイ状態はインスタンスごとに持つ {% #deployment-state %}

`current` と `blocked` のキーは、**そのインスタンスが**何をデプロイしたかを記録します。共有バケットではなく、cache backendのローカルディレクトリに保存されます。共有すると、artifactを公開したインスタンスの状態が他の全インスタンスからは「自ノードでデプロイ済み」に見えてしまい、そのバージョンを永久にスキップするためです。

{% callout type="warning" title="以前のリリースからのアップグレード" %}
S3とGCSを使うインスタンスは、これまで `current` をバケットに置いていました。アップグレード後、各インスタンスはローカルに記録が無い状態で起動するため、すでに動作しているバージョンをもう一度だけデプロイします。`dewy server` ではインスタンスあたり1回の再起動にあたります。以降のpollには影響しません。バケットに残る `current` オブジェクトは読まれなくなるので、任意のタイミングで削除できます。
{% /callout %}

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
  --interval 60 -- /opt/myapp/current/myapp

# S3/GCS共有キャッシュにより、各インスタンスが同じartifactを重複ダウンロードしないようにする
dewy server --registry ghr://owner/repo \
  --cache s3://ap-northeast-1/dewy-cache/myapp \
  --interval 30 -- /opt/myapp/current/myapp
```

> 補足: 共有キャッシュは上流registryへのartifactダウンロードトラフィックを削減します。各インスタンスのmetadata polling回数自体は減りません（こちらは別レイヤの課題で、registry層でのstale-while-revalidateとして将来対応予定）。

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

複数のDewyインスタンスを同じS3/GCS bucketに向けることで、artifactキャッシュを共有できます：

```sh
# AWS S3（endpoint optionでS3互換ストレージにも対応）
dewy server --registry ghr://owner/repo \
  --cache s3://ap-northeast-1/dewy-cache/myapp \
  --interval 60 -- /opt/myapp/current/myapp

# Google Cloud Storage
dewy server --registry ghr://owner/repo \
  --cache gs://dewy-cache/myapp \
  --interval 60 -- /opt/myapp/current/myapp
```

数十〜数百台のインスタンスが同じregistryをpollしても、新しいリリースを最初に検知したインスタンスだけがartifactダウンロードのコストを払い、残りは共有キャッシュから取得します。

## 関連項目 {% #related %}

- [アーキテクチャ](/ja/architecture) - Dewyの全体構成とキャッシュの位置づけ
- [レジストリ](/ja/registry) - アーティファクトの取得元設定
- [FAQ](/ja/faq) - キャッシュ関連のよくある質問
