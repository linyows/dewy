---
title: バージョニング
description: |
  Dewyは、セマンティックバージョニングまたはカレンダーバージョニングに基づいてアプリケーションの最新バージョンを自動検出し、
  継続的なデプロイメントを実現します。プリリリース版の管理も含めた包括的なバージョン管理機能を提供します。
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## 概要 {% #overview-details %}

Dewyにおけるバージョニングは、プル型デプロイメントの中核となる機能です。レジストリから取得したバージョン情報を基に、現在実行中のバージョンと比較して新しいバージョンが利用可能かを自動判定し、必要に応じて自動デプロイを実行します。

**主な特徴:**
- セマンティックバージョニング（SemVer）の完全サポート
- カレンダーバージョニング（CalVer）のサポート（柔軟なフォーマット指定子）
- プリリリース版の柔軟な管理
- 複数のバージョン形式に対応（`v1.2.3` / `1.2.3`）
- 環境別のバージョン戦略をサポート

## セマンティックバージョニング基礎 {% #semantic-versioning %}

### バージョン形式 {% #version-format %}

Dewyは、[Semantic Versioning 2.0.0](https://semver.org/lang/ja/)に準拠したバージョン管理をサポートします。

**基本形式:**
```
MAJOR.MINOR.PATCH
```

**例:**
- `1.2.3` - バージョン1.2.3
- `v1.2.3` - vプレフィックス付きバージョン1.2.3
- `2.0.0` - メジャーバージョン2.0.0

### バージョン番号の意味

{% table %}
* 種類
* 説明
* インクリメント条件
---
* MAJOR
* 後方互換性のない変更
* APIの破壊的変更、アーキテクチャ刷新
---
* MINOR
* 後方互換性のある機能追加
* 新機能追加、既存機能の拡張
---
* PATCH
* 後方互換性のあるバグ修正
* バグ修正、セキュリティ修正
{% /table %}

### プリリリースバージョン {% #pre-release %}

プリリリースバージョンは、正式リリース前のテストや評価を目的としたバージョンです。

**形式:**
```
MAJOR.MINOR.PATCH-<pre-release-identifier>
```

**よくあるパターン:**
- `v1.2.3-alpha` - アルファ版（初期テスト）
- `v1.2.3-beta.1` - ベータ版第1版（機能完成版のテスト）
- `v1.2.3-rc.1` - リリース候補第1版（最終確認版）

{% callout type="note" title="プリリリース版の優先順位" %}
プリリリース版は、同じMAJOR.MINOR.PATCHの正式版よりも低い優先度として扱われます。
例：`v1.2.3-rc.1 < v1.2.3`
{% /callout %}

### ビルドメタデータとデプロイメントスロット {% #build-metadata %}

セマンティックバージョニングでは、`+`記号で付加するビルドメタデータもサポートされています。Dewyはビルドメタデータを**デプロイメントスロット**管理に使用し、Blue/Greenデプロイメントパターンを実現します。

**形式:**
```
MAJOR.MINOR.PATCH+<build-metadata>
MAJOR.MINOR.PATCH-<pre-release>+<build-metadata>
```

**よくあるパターン:**
- `v1.2.3+blue` - Blueスロット用の安定版
- `v1.2.3+green` - Greenスロット用の安定版
- `v1.2.3-rc.1+blue` - Blueスロット用のプリリリース版

{% callout type="note" title="ビルドメタデータとバージョン比較" %}
セマンティックバージョニングの仕様により、ビルドメタデータはバージョン比較時に**無視**されます。
つまり、`v1.2.3+blue`と`v1.2.3+green`は**同じバージョン**とみなされます。
Dewyは`--slot`オプションを使用して、ビルドメタデータに基づいてデプロイ対象のバージョンをフィルタリングします。
{% /callout %}

**使用方法:**
```bash
# Blue環境 - +blueメタデータを持つバージョンのみをデプロイ
dewy server --registry ghr://owner/repo --slot blue -- /opt/myapp/current/myapp

# Green環境 - +greenメタデータを持つバージョンのみをデプロイ
dewy server --registry ghr://owner/repo --slot green -- /opt/myapp/current/myapp

# --slotなし - すべてのバージョンをデプロイ（後方互換）
dewy server --registry ghr://owner/repo -- /opt/myapp/current/myapp
```

## カレンダーバージョニング（CalVer） {% #calver %}

SemVerに加え、Dewyは[カレンダーバージョニング（CalVer）](https://calver.org/)もサポートしています。CalVerはリリース日に基づくバージョニング方式です。

### CalVerフォーマット {% #calver-format %}

`--calver`オプションでフォーマット文字列を指定すると、CalVerが有効になります。フォーマットはドットで区切られた指定子で構成されます。

**サポートされる指定子:**

{% table %}
* 指定子
* 説明
* 例
---
* YYYY
* フル年
* 2024
---
* YY
* 短縮年（パディングなし）
* 6, 16, 106
---
* 0Y
* ゼロ埋め短縮年
* 06, 16, 106
---
* MM
* 月（パディングなし）
* 1, 11
---
* 0M
* ゼロ埋め月
* 01, 11
---
* WW
* 週（パディングなし）
* 1, 33, 52
---
* 0W
* ゼロ埋め週
* 01, 33, 52
---
* DD
* 日（パディングなし）
* 1, 9, 31
---
* 0D
* ゼロ埋め日
* 01, 09, 31
---
* MICRO
* インクリメント番号
* 0, 1, 42
{% /table %}

**フォーマット例:**
- `YYYY.0M.0D.MICRO` - 年、ゼロ埋め月、ゼロ埋め日、マイクロ（例: `2024.01.15.3`）
- `YYYY.MM.DD` - 年、月、日（例: `2024.1.9`）
- `YYYY.0M.MICRO` - 年、ゼロ埋め月、マイクロ（例: `2024.06.3`）

### CalVerの使い方 {% #calver-usage %}

```bash
# GitHub ReleasesでCalVerを使用
dewy server --registry ghr://owner/repo --calver YYYY.0M.0D.MICRO -- /opt/myapp/current/myapp

# S3でCalVerを使用
dewy server --registry "s3://ap-northeast-1/releases/myapp" --calver YYYY.0M.MICRO -- /opt/myapp/current/myapp

# CalVerでプリリリース版を含む
dewy server --registry "ghr://owner/repo?pre-release=true" --calver YYYY.0M.0D.MICRO -- /opt/myapp/current/myapp
```

### CalVerでのプリリリースとビルドメタデータ {% #calver-metadata %}

CalVerはSemVerと同様に、プリリリース識別子とビルドメタデータをサポートしています：

```
<calver>-<pre-release>+<build-metadata>
```

**例:**
- `2024.01.15.3-rc.1` - リリース候補
- `2024.06.0+blue` - Blueデプロイメントスロット
- `v2024.01.15.3-beta.2+green` - Greenスロット用のプリリリース（vプレフィックス付き）

{% callout type="note" title="CalVerとBlue/Greenデプロイメント" %}
ビルドメタデータ（`+blue`、`+green`）とプリリリース識別子は、SemVerとCalVerの両方で同じように機能します。Blue/Green、ステージング、カナリアなどのすべてのデプロイメントパターンがCalVerで完全にサポートされています。
{% /callout %}

## Dewyのバージョン検出アルゴリズム {% #version-detection %}

### バージョン比較規則 {% #comparison-rules %}

DewyはSemVerとCalVerの両方に対応したバージョン比較アルゴリズムを実装しています：

**SemVer比較:**

1. **MAJOR版の比較** - 数値として比較し、大きい方を優先
2. **MINOR版の比較** - MAJOR版が同じ場合、数値として比較
3. **PATCH版の比較** - MAJOR.MINORが同じ場合、数値として比較
4. **プリリリース版の処理** - 正式版 > プリリリース版、プリリリース版同士は文字列比較

**CalVer比較:**

1. **セグメントごとの比較** - 各セグメントを左から順に数値として比較
2. **プリリリース版の処理** - SemVerと同じ: 正式版 > プリリリース版

### 最新バージョンの決定 {% #latest-version %}

レジストリから取得したすべてのバージョンタグに対して：

```go
// 擬似コード
func findLatest(versions []string, allowPreRelease bool, calverFormat string) string {
    if calverFormat != "" {
        validVersions := filterValidCalVer(versions, calverFormat, allowPreRelease)
        return findMaxVersion(validVersions)
    }
    validVersions := filterValidSemVer(versions, allowPreRelease)
    return findMaxVersion(validVersions)
}
```

**処理フロー:**
1. バージョン形式の検証（`--calver`オプションに基づきSemVerまたはCalVer）
2. プリリリース設定によるフィルタリング
3. 数値による比較とソート
4. 最大値の選択

## レジストリ別バージョン管理 {% #registry-versioning %}

### GitHub Releases {% #github-releases %}

GitHubリリースのタグ名から自動的にバージョンを検出します。

```bash
# 安定版のみ（デフォルト）
dewy server --registry ghr://owner/repo

# プリリリース版を含む
dewy server --registry "ghr://owner/repo?pre-release=true"

# CalVerフォーマットを使用
dewy server --registry ghr://owner/repo --calver YYYY.0M.0D.MICRO
```

**グレースピリオドの考慮:**

{% callout type="important" title="CI/CD対応" %}
GitHub Actionsなどでリリース作成後、アーティファクトのビルドと配置に時間がかかる場合があります。
Dewyは新しいリリースについては30分間のグレースピリオドを設け、この間は「アーティファクトが見つからない」エラーを通知しません。
{% /callout %}

### AWS S3 {% #aws-s3 %}

S3のオブジェクトパス構造からバージョンを抽出します。

**必須パス構造:**
```
<path-prefix>/<version>/<artifact>
```

**設定例:**
```bash
# SemVer（デフォルト）
dewy server --registry "s3://ap-northeast-1/releases/myapp?pre-release=true"

# CalVer
dewy server --registry "s3://ap-northeast-1/releases/myapp" --calver YYYY.0M.MICRO
```

**S3内の配置例（SemVer）:**
```
releases/myapp/v1.2.4/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.4/myapp_darwin_arm64.tar.gz
releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.3-rc.1/myapp_linux_amd64.tar.gz
```

**S3内の配置例（CalVer）:**
```
releases/myapp/2024.06.15.0/myapp_linux_amd64.tar.gz
releases/myapp/2024.06.15.1/myapp_linux_amd64.tar.gz
releases/myapp/2024.07.01.0/myapp_linux_amd64.tar.gz
```

### Google Cloud Storage {% #google-cloud-storage %}

Google Cloud StorageもS3と同様のパス構造でバージョン管理を行います。

```bash
dewy server --registry "gs://my-bucket/releases/myapp?pre-release=false"
```

### gRPC {% #grpc %}

gRPCレジストリでは、サーバー側でバージョン情報を管理します。

```bash
dewy server --registry "grpc://registry-server:9000"
```

{% callout type="note" %}
gRPCレジストリでは`pre-release`オプションは使用できません。サーバー側の実装に依存します。
{% /callout %}

## 環境別バージョン戦略 {% #environment-strategies %}

### 本番環境 {% #production %}

**推奨設定:**
```bash
# 安定版のみを自動デプロイ
dewy server --registry ghr://company/myapp \
  --interval 300s \
  --log-format json -- /opt/myapp/current/myapp
```

**特徴:**
- プリリリース版は除外（`pre-release=false`）
- 長めのポーリング間隔でシステム負荷を軽減
- 構造化ログでモニタリングしやすさを優先

### ステージング環境 {% #staging %}

**推奨設定:**
```bash
# プリリリース版も含めて早期テスト
dewy server --registry "ghr://company/myapp?pre-release=true" \
  --interval 60s \
  --notifier "slack://staging-deploy?title=MyApp+Staging" \
  -- /opt/myapp/current/myapp
```

**特徴:**
- プリリリース版を積極的に取り込み
- 短いポーリング間隔で迅速なフィードバック
- デプロイ通知でチーム全体に共有

### 開発環境 {% #development %}

**推奨設定:**
```bash
# 最新の開発版を即座に反映
dewy server --registry "ghr://company/myapp?pre-release=true" \
  --interval 30s \
  --log-format text -- ./current/myapp
```

### Blue/Greenデプロイメント {% #blue-green %}

Blue/Greenデプロイメントパターンでは、ビルドメタデータを使用してデプロイメントスロットを管理します：

**推奨設定:**
```bash
# Blue環境
dewy server --registry ghr://company/myapp --slot blue \
  --interval 60s \
  --notifier "slack://production-deploy?title=MyApp+Blue" \
  -- /opt/myapp/current/myapp

# Green環境
dewy server --registry ghr://company/myapp --slot green \
  --interval 60s \
  --notifier "slack://production-deploy?title=MyApp+Green" \
  -- /opt/myapp/current/myapp
```

**特徴:**
- 各環境で独立したバージョン管理
- トラフィック切り替えによるゼロダウンタイムデプロイメント
- トラフィックを戻すだけで簡単にロールバック
- プリリリースと組み合わせてカナリアデプロイメントも可能

**デプロイメントワークフロー:**
```bash
# Step 1: Green（スタンバイ）にデプロイ
gh release create v1.2.0+green --title "v1.2.0 for Green"

# Step 2: Green環境の動作確認

# Step 3: トラフィックをGreenに切り替え（ロードバランサー経由）

# Step 4: Blueにも同じバージョンをデプロイ
gh release create v1.2.0+blue --title "v1.2.0 for Blue"

# 両環境がv1.2.0で稼働
```

## バージョン管理のベストプラクティス {% #best-practices %}

### タグ付けルール {% #tagging-rules %}

**推奨するタグ命名規則:**

```bash
# 正式リリース
git tag v1.2.3
git tag v2.0.0

# プリリリース
git tag v1.3.0-alpha
git tag v1.3.0-beta.1
git tag v1.3.0-rc.1

# セキュリティ修正
git tag v1.2.4  # 1.2.3のセキュリティ修正版
```

**避けるべきパターン:**
```bash
# ❌ 構造化されていない命名
git tag latest
git tag stable

# ❌ 不規則な命名
git tag v1.2.3-SNAPSHOT
git tag 1.2.3-final
```

{% callout type="note" title="日時ベースのタグ" %}
`2024.03.15.0`のような日付ベースのタグは、`--calver`オプションを使用してサポートされています。詳しくは[カレンダーバージョニング](#calver)を参照してください。
{% /callout %}

### リリース戦略 {% #release-strategy %}

**段階的リリースパターン:**

1. **alpha** - 内部開発者によるテスト
2. **beta** - 限定ユーザーによるテスト
3. **rc** (Release Candidate) - 本番環境に近い条件でのテスト
4. **正式版** - 本番環境への展開

**例:**
```bash
v2.1.0-alpha    → 開発環境
v2.1.0-beta.1   → ステージング環境
v2.1.0-rc.1     → ステージング環境（本番同等構成）
v2.1.0          → 本番環境
```

## 設定例とパターン {% #configuration-patterns %}

### CI/CDとの連携パターン {% #cicd-integration %}

**GitHub Actions との連携例:**

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ['v*']
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build and Release
        run: |
          make build
          gh release create ${{ github.ref_name }} \
            --title "Release ${{ github.ref_name }}" \
            --generate-notes \
            dist/*
```

**ステージング環境での自動テスト:**

```bash
# プリリリース版を監視してE2Eテストを自動実行
dewy server --registry "ghr://company/myapp?pre-release=true" \
  --after-deploy-hook "make e2e-test" \
  -- /opt/myapp/current/myapp
```

## トラブルシューティング {% #troubleshooting %}

### よくある問題と解決方法 {% #common-issues %}

**バージョンが検出されない:**

```bash
# デバッグ用：利用可能なタグを確認
curl -s https://api.github.com/repos/owner/repo/releases \
  | jq -r '.[].tag_name'

# ログで検出プロセスを確認
dewy server --log-format json -l debug --registry ghr://owner/repo
```

**想定外のバージョンが選択される:**

```bash
# プリリリース設定の確認
dewy server --registry "ghr://owner/repo?pre-release=false"  # 安定版のみ
dewy server --registry "ghr://owner/repo?pre-release=true"   # プリリリース込み
```

**アクセス権限の問題:**

```bash
# GitHub Tokenの確認
echo $GITHUB_TOKEN | cut -c1-10  # 最初の10文字のみ表示
gh auth status  # GitHub CLI での認証状態確認
```

### 日時ベースのタグを使用するには {% #datetime-tags %}

{% callout type="note" title="CalVerの使用を推奨" %}
日時ベースのタグを使用する場合は、`--calver`オプションを指定してください。CalVerフォーマットを使用することで、ゼロ埋めの月・日も正しくパースされます。

**CalVerを使用した例:**
```bash
# ✅ CalVerフォーマットで正しく動作
dewy server --registry ghr://owner/repo --calver YYYY.0M.0D.MICRO

# タグの例
2025.09.05.0   # 2025年9月5日のリリース
2025.01.01.0   # 2025年1月1日のリリース
2025.12.25.0   # 2025年12月25日のリリース
```

`--calver`オプションなしで日時ベースのタグを使用すると、SemVerとしてパースされるため意図しない結果になることがあります。
{% /callout %}

### アップグレードの問題 {% #upgrade-issues %}

**メジャーバージョンアップ対応:**

```bash
# データ移行を含む場合のフック活用
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "scripts/migrate-data.sh" \
  --after-deploy-hook "scripts/validate-upgrade.sh" \
  -- /opt/myapp/current/myapp
```

**ロールバック手順:**

```bash
# 手動でのロールバック
cd /opt/myapp
rm current
ln -sf releases/20241201T120000Z current  # 前のバージョンに戻す
systemctl restart myapp
```

## 高度な使用例 {% #advanced-usage %}

### 複数環境での段階的展開 {% #staged-deployment %}

**開発 → ステージング → 本番の自動化:**

```bash
# 開発環境：すべてのプリリリースを即座にデプロイ
dewy server --registry "ghr://company/myapp?pre-release=true" \
  --interval 30s

# ステージング環境：RC版以上をデプロイ
# （将来的な機能として、フィルタリングオプションの追加を検討）

# 本番環境：安定版のみを慎重にデプロイ
dewy server --registry "ghr://company/myapp?pre-release=false" \
  --interval 600s \
  --before-deploy-hook "scripts/pre-deployment-check.sh"
```

### カスタムバージョンパターン {% #custom-patterns %}

Dewyはセマンティックバージョニング（SemVer）とカレンダーバージョニング（CalVer）の両方をサポートしています。CalVerの柔軟なフォーマット指定子を使用することで、日付ベースの命名規則に対応できます。詳しくは[カレンダーバージョニング](#calver)を参照してください。

## 関連項目 {% #related %}

- [レジストリ](/ja/registry) - バージョン検出元の設定と各レジストリの詳細
- [キャッシュ](/ja/cache) - バージョン情報とアーティファクトの保存管理
- [アーキテクチャ](/ja/architecture) - Dewyの全体構成とデプロイプロセス
- [FAQ](/ja/faq) - バージョニング関連のよくある質問