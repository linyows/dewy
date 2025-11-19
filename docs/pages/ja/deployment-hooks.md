---
title: デプロイメントフック
description: |
  デプロイメントフックは、デプロイの前後にカスタムコマンドを実行する機能です。
  データベースバックアップ、サービス管理、通知送信など、デプロイプロセスを柔軟にカスタマイズできます。
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## 概要 {% #overview-details %}

デプロイメントフックは、Dewyの自動デプロイプロセスをカスタマイズするための強力な機能です。アプリケーションのデプロイ前後に任意のシェルコマンドを実行でき、データベースの操作、外部サービスとの連携、検証処理など、様々な用途に活用できます。

**主な特徴:**
- **柔軟な実行タイミング**: デプロイ前後での実行制御
- **完全な環境アクセス**: 環境変数とファイルシステムへのフルアクセス
- **詳細な実行結果**: stdout/stderr、終了コード、実行時間の記録
- **通知連携**: 設定済み通知チャネルへの実行結果送信

## フックの種類と動作 {% #hook-types %}

### Before Deploy Hook {% #before-hook %}

デプロイが開始される**前**に実行されるフックです。

**実行タイミング:**
- アーティファクトダウンロード後
- ファイル展開とシンボリックリンク作成前
- アプリケーション再起動前

**重要な動作:**
```bash
# Before Hookが失敗した場合、デプロイは中止される
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "scripts/pre-check.sh" \
  -- /opt/myapp/current/myapp
```

{% callout type="warning" title="デプロイ中止の条件" %}
Before Deploy Hookが0以外の終了コードで終了した場合、デプロイプロセス全体が中止されます。
この動作により、事前条件が満たされていない場合の安全なデプロイ防止が可能です。
{% /callout %}

### After Deploy Hook {% #after-hook %}

デプロイが**成功**した後に実行されるフックです。

**実行タイミング:**
- ファイル展開とシンボリックリンク作成完了後
- アプリケーション再起動完了後（server コマンドの場合）
- デプロイプロセスの最終段階

**重要な動作:**
```bash
# After Hookが失敗してもデプロイは成功として扱われる
dewy server --registry ghr://owner/repo \
  --after-deploy-hook "scripts/post-deploy-validation.sh" \
  -- /opt/myapp/current/myapp
```

{% callout type="note" %}
After Deploy Hookの失敗はデプロイの成功ステータスに影響しません。
ただし、エラーはログに記録され、設定されている場合は通知が送信されます。
{% /callout %}

## 実行環境と制約 {% #execution-environment %}

### 実行環境 {% #environment %}

フックは以下の環境で実行されます：

**シェル実行:**
```bash
/bin/sh -c "your-command"
```

**作業ディレクトリ:**
- Dewyの実行ディレクトリ（通常はアプリケーションのルートディレクトリ）

**環境変数:**
- Dewyプロセスのすべての環境変数を継承
- 実行時の環境変数へのフルアクセス

### 実行結果の取得 {% #execution-results %}

フック実行時に以下の情報が記録されます：

{% table %}
* 項目
* 説明
* 用途
---
* Command
* 実行されたコマンド文字列
* デバッグとログ記録
---
* Stdout
* 標準出力の内容
* 実行結果の確認
---
* Stderr
* 標準エラー出力
* エラー内容の把握
---
* ExitCode
* プロセス終了コード
* 成功/失敗の判定
---
* Duration
* 実行時間
* パフォーマンス監視
{% /table %}

**ログ出力例:**
```json
{
  "time": "2024-03-15T10:30:45Z",
  "level": "INFO",
  "msg": "Execute hook success",
  "command": "backup-database.sh",
  "stdout": "Backup completed successfully",
  "stderr": "",
  "exit_code": 0,
  "duration": "2.5s"
}
```

## 設定方法 {% #configuration %}

### コマンドライン設定 {% #command-line %}

```bash
# 基本形式
dewy server --registry <registry-url> \
  --before-deploy-hook "<command>" \
  --after-deploy-hook "<command>" \
  -- <application-command>
```

### 設定例 {% #configuration-examples %}

**単純なコマンド実行:**
```bash
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "echo 'Starting deployment'" \
  --after-deploy-hook "echo 'Deployment completed'" \
  -- /opt/myapp/current/myapp
```

**複数コマンドの連携:**
```bash
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "systemctl stop nginx && backup-db.sh" \
  --after-deploy-hook "systemctl start nginx && send-notification.sh" \
  -- /opt/myapp/current/myapp
```

**スクリプトファイルの実行:**
```bash
dewy server --registry ghr://owner/repo \
  --before-deploy-hook "/opt/scripts/pre-deploy.sh" \
  --after-deploy-hook "/opt/scripts/post-deploy.sh" \
  -- /opt/myapp/current/myapp
```

## 実用的なユースケース {% #use-cases %}

### データベース操作 {% #database-operations %}

**バックアップの自動実行:**
```bash
# PostgreSQL バックアップ
--before-deploy-hook "pg_dump myapp_db > /backup/myapp_$(date +%Y%m%d_%H%M%S).sql"

# MySQL バックアップ
--before-deploy-hook "mysqldump -u root -p myapp_db > /backup/myapp_$(date +%Y%m%d_%H%M%S).sql"
```

**マイグレーションの実行:**
```bash
# Rails マイグレーション
--after-deploy-hook "cd /opt/myapp/current && bundle exec rake db:migrate"

# Django マイグレーション
--after-deploy-hook "cd /opt/myapp/current && python manage.py migrate"

# Go マイグレーション（migrate ツール使用）
--after-deploy-hook "migrate -path /opt/myapp/current/migrations -database 'postgres://...' up"
```

### サービス管理 {% #service-management %}

**関連サービスの制御:**
```bash
# Nginx の一時停止と再開
--before-deploy-hook "systemctl stop nginx"
--after-deploy-hook "systemctl start nginx && systemctl reload nginx"

# ロードバランサーからの切り離し
--before-deploy-hook "curl -X DELETE http://lb:8080/servers/$(hostname)"
--after-deploy-hook "curl -X POST http://lb:8080/servers/$(hostname)"
```

**ヘルスチェックの実行:**
```bash
# アプリケーションの起動確認
--after-deploy-hook "timeout 30 bash -c 'until curl -f http://localhost:8080/health; do sleep 1; done'"

# データベース接続確認
--before-deploy-hook "pg_isready -h localhost -p 5432 -d myapp_db"
```

### 通知とモニタリング {% #notification-monitoring %}

**外部システムへの通知:**
```bash
# Datadog へのデプロイイベント送信
--after-deploy-hook "curl -X POST https://api.datadoghq.com/api/v1/events \
  -H 'DD-API-KEY: ${DD_API_KEY}' \
  -d '{\"title\":\"Deployment\",\"text\":\"App deployed\"}'"

# PagerDuty への通知
--after-deploy-hook "scripts/notify-pagerduty.sh deployment-success"
```

**メトリクス収集:**
```bash
# デプロイ時間の記録
--before-deploy-hook "echo $(date +%s) > /tmp/deploy_start"
--after-deploy-hook "echo 'Deploy time: '$(($(date +%s) - $(cat /tmp/deploy_start)))'s'"
```

### 設定とファイル管理 {% #configuration-management %}

**設定ファイルの更新:**
```bash
# 環境固有の設定適用
--after-deploy-hook "cp /opt/config/production.yml /opt/myapp/current/config.yml"

# テンプレートから設定生成
--after-deploy-hook "envsubst < /opt/templates/app.conf.template > /opt/myapp/current/app.conf"
```

**キャッシュのクリア:**
```bash
# Redis キャッシュクリア
--after-deploy-hook "redis-cli FLUSHALL"

# ファイルキャッシュクリア
--after-deploy-hook "rm -rf /opt/myapp/current/tmp/cache/*"

# CDN キャッシュ無効化
--after-deploy-hook "curl -X POST 'https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/purge_cache' \
  -H 'Authorization: Bearer ${CF_TOKEN}' -d '{\"purge_everything\":true}'"
```

## エラーハンドリングと復旧 {% #error-handling %}

### Before Hook失敗時 {% #before-hook-failure %}

Before Hookが失敗した場合の動作：

1. **デプロイの自動中止**: プロセス全体が停止
2. **エラーログの記録**: 詳細な実行結果をログに出力
3. **通知の送信**: 設定されている場合、エラー通知を送信
4. **現在の状態維持**: 既存のアプリケーションは影響を受けない

**推奨される復旧手順:**
```bash
# 1. エラー原因の確認
tail -f /var/log/dewy.log

# 2. 手動でのフック実行テスト
/bin/sh -c "your-before-hook-command"

# 3. 問題修正後のデプロイ再試行
# Dewyは次回のポーリングで自動的に再試行
```

### After Hook失敗時 {% #after-hook-failure %}

After Hookが失敗した場合の動作：

1. **デプロイは成功として処理**: アプリケーションは新バージョンで稼働
2. **エラーログの記録**: 失敗内容を詳細にログ出力
3. **通知の送信**: 設定されている場合、警告通知を送信
4. **手動対応の推奨**: 管理者による確認と対応が必要

**推奨される対応手順:**
```bash
# 1. 新バージョンの動作確認
curl -f http://localhost:8080/health

# 2. After Hookの手動実行
/bin/sh -c "your-after-hook-command"

# 3. 必要に応じて手動での後処理実行
```

## セキュリティ考慮事項 {% #security %}

### 権限管理 {% #permission-management %}

**実行ユーザーの権限設定:**
```bash
# Dewyを専用ユーザーで実行
sudo useradd -r -s /bin/bash dewy
sudo chown -R dewy:dewy /opt/myapp

# 必要最小限の権限でサービス定義
# /etc/systemd/system/dewy.service
[Service]
User=dewy
Group=dewy
```

**sudo使用時の注意点:**
```bash
# ❌ 危険：パスワードプロンプトでハング
--before-deploy-hook "sudo systemctl stop nginx"

# ✅ 安全：NOPASSWDまたは専用ユーザー権限設定
--before-deploy-hook "systemctl stop nginx"  # systemd user session
```

### コマンドインジェクション対策 {% #injection-prevention %}

**安全なコマンド記述:**
```bash
# ✅ 安全：クォートによる保護
--before-deploy-hook "backup-db.sh --name 'myapp_backup'"

# ❌ 危険：環境変数の直接展開
--before-deploy-hook "echo $USER_INPUT"

# ✅ 安全：環境変数の適切な使用
--before-deploy-hook "scripts/safe-command.sh"  # スクリプト内で適切に処理
```

**推奨される実装パターン:**
```bash
#!/bin/bash
# scripts/safe-backup.sh
set -euo pipefail

DB_NAME="${DB_NAME:-myapp}"
BACKUP_DIR="${BACKUP_DIR:-/backup}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

pg_dump "${DB_NAME}" > "${BACKUP_DIR}/backup_${TIMESTAMP}.sql"
```

## パフォーマンスと監視 {% #performance-monitoring %}

### 実行時間の考慮 {% #execution-time %}

**長時間実行コマンドの対策:**
```bash
# タイムアウト設定
--before-deploy-hook "timeout 300 large-backup.sh"

# バックグラウンド実行（注意：デプロイ完了を待たない）
--after-deploy-hook "nohup long-running-task.sh &"

# 非同期処理への委譲
--after-deploy-hook "queue-job.sh heavy-processing"
```

### 監視とデバッグ {% #monitoring-debug %}

**詳細ログの有効化:**
```bash
# デバッグレベルでの実行
dewy server --log-format json --log-level debug \
  --registry ghr://owner/repo \
  --before-deploy-hook "verbose-script.sh"
```

**パフォーマンス測定:**
```bash
# 実行時間測定付きのフック
--before-deploy-hook "time backup-database.sh"
--after-deploy-hook "time validate-deployment.sh"
```

## 設定例とベストプラクティス {% #best-practices %}

### 本番環境での推奨設定 {% #production-settings %}

**安全なバックアップ戦略:**
```bash
dewy server --registry ghr://company/myapp \
  --before-deploy-hook "scripts/production-backup.sh" \
  --after-deploy-hook "scripts/production-validation.sh" \
  --notifier "slack://ops-alerts" \
  -- /opt/myapp/current/myapp
```

**production-backup.sh の例:**
```bash
#!/bin/bash
set -euo pipefail

# データベースバックアップ
pg_dump myapp_production > "/backup/pre-deploy-$(date +%Y%m%d_%H%M%S).sql"

# 設定ファイルバックアップ
cp /opt/myapp/current/config.yml "/backup/config-$(date +%Y%m%d_%H%M%S).yml"

# ヘルスチェック
curl -f http://localhost:8080/health || exit 1

echo "Pre-deployment backup completed successfully"
```

### 開発・ステージング環境 {% #development-staging %}

**開発効率重視の設定:**
```bash
dewy server --registry "ghr://company/myapp?pre-release=true" \
  --interval 30s \
  --before-deploy-hook "scripts/dev-prepare.sh" \
  --after-deploy-hook "scripts/dev-test.sh" \
  --notifier "slack://dev-deploys" \
  -- /opt/myapp/current/myapp
```

**自動テスト連携:**
```bash
#!/bin/bash
# scripts/dev-test.sh
set -euo pipefail

# アプリケーション起動待機
sleep 5

# ヘルスチェック
curl -f http://localhost:8080/health

# E2E テスト実行
cd /opt/myapp/current && npm test

echo "Development validation completed"
```

## 高度な活用例 {% #advanced-usage %}

### ローリングアップデート {% #rolling-update %}

**トラフィック切り替えの自動化:**
```bash
# ローリングアップデート用のフック設定
dewy server --registry ghr://company/myapp \
  --before-deploy-hook "scripts/prepare-green.sh" \
  --after-deploy-hook "scripts/switch-to-green.sh" \
  -- /opt/myapp/current/myapp
```

**switch-to-green.sh の例:**
```bash
#!/bin/bash
set -euo pipefail

# Green環境のヘルスチェック
for i in {1..30}; do
  if curl -f http://localhost:8081/health; then
    break
  fi
  sleep 2
done

# ロードバランサーの切り替え
curl -X POST http://lb:8080/switch-to-green

# Blue環境の停止（遅延実行）
sleep 30 && systemctl stop myapp-blue &

echo "Traffic switched to green environment"
```

### CI/CDパイプライン連携 {% #cicd-integration %}

**GitHub Actions との連携:**
```bash
# デプロイ完了をGitHub Actionsに通知
--after-deploy-hook "scripts/notify-github-actions.sh"
```

**notify-github-actions.sh の例:**
```bash
#!/bin/bash
set -euo pipefail

# GitHub の Deployment Status API を使用
curl -X POST \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/owner/repo/deployments/${DEPLOYMENT_ID}/statuses \
  -d '{"state":"success","description":"Deployed successfully via Dewy"}'
```

## トラブルシューティング {% #troubleshooting %}

### よくある問題 {% #common-issues %}

**権限エラー:**
```bash
# 問題：Permission denied
--before-deploy-hook "systemctl stop nginx"

# 解決策：ユーザー権限の確認と調整
sudo usermod -a -G sudo dewy
# または
sudo visudo  # NOPASSWDの設定
```

**パス問題:**
```bash
# 問題：command not found
--after-deploy-hook "npm install"

# 解決策：フルパスまたはPATH設定
--after-deploy-hook "/usr/local/bin/npm install"
# または
--after-deploy-hook "PATH=/usr/local/bin:$PATH npm install"
```

**環境変数の問題:**
```bash
# 問題：Environment variable not found
--before-deploy-hook "echo $CUSTOM_VAR"

# 解決策：環境変数の事前設定確認
systemctl edit dewy.service
# [Service]
# Environment=CUSTOM_VAR=value
```

### デバッグ手法 {% #debugging %}

**段階的な問題切り分け:**
```bash
# 1. シンプルなコマンドから開始
--before-deploy-hook "echo 'Hook test'"

# 2. 段階的に複雑化
--before-deploy-hook "echo 'Hook test' && date"

# 3. 実際のコマンド
--before-deploy-hook "your-actual-command"
```

**手動実行での検証:**
```bash
# Dewyと同じ環境でテスト
cd /opt/myapp
sudo -u dewy /bin/sh -c "your-hook-command"
```

**ログ分析:**
```bash
# フック関連ログの抽出
journalctl -u dewy.service | grep -i hook

# JSON形式ログの解析
journalctl -u dewy.service -o json | jq 'select(.msg | contains("hook"))'
```

## 通知連携 {% #notification-integration %}

### フック実行結果の自動通知 {% #hook-result-notification %}

デプロイメントフックの実行結果は、設定された通知チャネル（Slack/Mail）に自動的に送信されます。

**通知される情報:**
- 実行されたコマンド
- 実行結果（成功/失敗）
- 標準出力・エラー出力
- 実行時間
- 終了コード

### 通知設定との組み合わせ例 {% #notification-examples %}

**Slack通知付きの設定:**
```bash
dewy server --registry ghr://owner/repo \
  --notifier "slack://deploy-channel?title=MyApp" \
  --before-deploy-hook "scripts/backup-database.sh" \
  --after-deploy-hook "scripts/validate-deployment.sh" \
  -- /opt/myapp/current/myapp
```

**Mail通知付きの設定:**
```bash
dewy server --registry ghr://owner/repo \
  --notifier "smtp://smtp.company.com:587/ops@company.com" \
  --before-deploy-hook "scripts/pre-deploy-check.sh" \
  --after-deploy-hook "scripts/post-deploy-report.sh" \
  -- /opt/myapp/current/myapp
```

{% callout type="note" %}
通知チャネルの詳細な設定方法については、[通知](/ja/notifier)ドキュメントを参照してください。
{% /callout %}

## 関連項目 {% #related %}

- [アーキテクチャ](/ja/architecture) - デプロイプロセス全体でのフックの位置づけ
- [通知](/ja/notifier) - 通知チャネルの詳細設定（Slack/Mail）
- [バージョニング](/ja/versioning) - デプロイトリガーとしてのバージョン検出
- [FAQ](/ja/faq) - デプロイメントフック関連のよくある質問