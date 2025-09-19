---
title: バージョニング
description: |
  Dewyは、セマンティックバージョニングに基づいてアプリケーションの最新バージョンを自動検出し、
  継続的なデプロイメントを実現します。プリリリース版の管理も含めた包括的なバージョン管理機能を提供します。
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## 概要 {% #overview-details %}

Dewyにおけるバージョニングは、プル型デプロイメントの中核となる機能です。レジストリから取得したバージョン情報を基に、現在実行中のバージョンと比較して新しいバージョンが利用可能かを自動判定し、必要に応じて自動デプロイを実行します。

**主な特徴:**
- セマンティックバージョニング（SemVer）の完全サポート
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

## Dewyのバージョン検出アルゴリズム {% #version-detection %}

### バージョン比較規則 {% #comparison-rules %}

Dewyは独自のセマンティックバージョン比較アルゴリズムを実装しています：

1. **MAJOR版の比較** - 数値として比較し、大きい方を優先
2. **MINOR版の比較** - MAJOR版が同じ場合、数値として比較
3. **PATCH版の比較** - MAJOR.MINORが同じ場合、数値として比較
4. **プリリリース版の処理**:
   - 正式版 > プリリリース版
   - プリリリース版同士は文字列比較

### 最新バージョンの決定 {% #latest-version %}

レジストリから取得したすべてのバージョンタグに対して：

```go
// 擬似コード
func findLatest(versions []string, allowPreRelease bool) string {
    validVersions := filterValidSemVer(versions, allowPreRelease)
    return findMaxVersion(validVersions)
}
```

**処理フロー:**
1. セマンティックバージョン形式の検証
2. プリリリース設定による フィルタリング
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
<path-prefix>/<semver>/<artifact>
```

**設定例:**
```bash
dewy server --registry "s3://ap-northeast-1/releases/myapp?pre-release=true"
```

**S3内の配置例:**
```
releases/myapp/v1.2.4/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.4/myapp_darwin_arm64.tar.gz
releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.3-rc.1/myapp_linux_amd64.tar.gz
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
# ❌ セマンティックバージョニングに非準拠
git tag release-2024-03-15
git tag latest
git tag stable

# ❌ 不規則な命名
git tag v1.2.3-SNAPSHOT
git tag 1.2.3-final
```

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

### 日時ベースのタグ使用時の注意 {% #datetime-tags-warning %}

{% callout type="warning" title="日時ベースのタグ使用時の注意" %}
Dewyはタグを数値として比較するため、日時ベースのタグを使用する場合は注意が必要です。

**問題のあるパターン:**
```bash
# ❌ 先頭の0が意図しない結果を招く
v2025.0905.1005  # v2025.905.1005 として認識され、比較が正しく動作しない
v2025.0101.0800  # v2025.101.800 として認識される
```

**推奨するパターン:**
```bash
# ✅ 先頭の0を除いた形式
v2025.905.1005   # 9月5日 10時05分
v2025.101.800    # 1月1日 8時00分
v2025.1225.1500  # 12月25日 15時00分
```

または、セマンティックバージョニングに準拠した形式を使用してください：
```bash
# ✅ セマンティックバージョニング準拠
v1.0.0, v1.1.0, v2.0.0
```
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

現在のDewyは標準的なセマンティックバージョニングのみをサポートしていますが、
将来的には企業独自の命名規則にも対応予定です。

## 関連項目 {% #related %}

- [レジストリ](/ja/registry) - バージョン検出元の設定と各レジストリの詳細
- [キャッシュ](/ja/cache) - バージョン情報とアーティファクトの保存管理
- [アーキテクチャ](/ja/architecture) - Dewyの全体構成とデプロイプロセス
- [FAQ](/ja/faq) - バージョニング関連のよくある質問