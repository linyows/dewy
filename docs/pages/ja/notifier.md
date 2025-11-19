---
title: 通知
description: |
  通知機能は、デプロイメントの状況をチームに自動で伝えるDewyのコンポーネントです。
  成功・失敗・フック実行結果など、様々なイベントをSlackやメールで通知できます。
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## 対応通知方法

Dewyは以下の通知方法に対応しています。

- **Slack** (`slack://`): Slackチャンネルへの通知
- **Mail** (`smtp://`): SMTP経由でのメール通知

## 通知のタイミング

Dewyは以下のタイミングで通知を送信します：

- **開始時**: Dewyサービスの開始
- **ダウンロード完了**: 新しいアーティファクトのダウンロード
- **デプロイ成功**: アプリケーションの起動・再起動成功
- **エラー発生**: 各種エラーの発生
- **フック実行**: Before/Afterフックの実行結果
- **停止時**: Dewyサービスの停止

## Slack通知

基本設定

```bash
# 基本形式
slack://<channel-name>

# 例
dewy server --registry ghr://owner/repo \
  --notifier slack://deployments \
  -- /opt/myapp/current/myapp
```

環境変数

```bash
# Slack Bot Token（必須）
export SLACK_TOKEN=xoxb-xxxxxxxxxxxxxxxxxxxxx
```

### Slack Appの設定

1. Slack Appの作成
   - [https://api.slack.com/apps](https://api.slack.com/apps) でアプリを作成
2. 必要な権限（Scopes）
   - `channels:join`: チャンネルへの参加
   - `chat:write`: メッセージの投稿
3. トークンの取得
   - OAuth & Permissions → Bot User OAuth Token

### オプション付きの設定

```bash
# タイトル付き通知
dewy server --registry ghr://owner/repo \
  --notifier "slack://deployments?title=MyApp"

# URL付き通知（リポジトリへのリンク等）
dewy server --registry ghr://owner/repo \
  --notifier "slack://deployments?title=MyApp&url=https://github.com/owner/repo"

# 複数オプション
dewy server --registry ghr://owner/repo \
  --notifier "slack://prod-deploy?title=MyApp&url=https://myapp.example.com"
```

### 通知内容例

```
🚀 Automatic shipping started by Dewy (v1.2.3: server)

✅ Downloaded artifact for v1.2.3

🔄 Server restarted for v1.2.3

❌ Deploy failed: connection timeout
```

## メール通知

基本設定

```bash
# 基本形式
smtp://<smtp-host>:<port>/<recipient>

# 例
dewy server --registry ghr://owner/repo \
  --notifier smtp://smtp.gmail.com:587/admin@example.com \
  -- /opt/myapp/current/myapp
```

環境変数

```bash
# SMTP認証情報
export MAIL_USERNAME=sender@gmail.com
export MAIL_PASSWORD=app-specific-password
export MAIL_FROM=sender@gmail.com
```

### 設定オプション

{% table %}
* オプション
* 型
* 説明
* デフォルト値
---
* `username`
* string
* SMTP認証ユーザー名
* MAIL_USERNAME環境変数
---
* `password`
* string
* SMTP認証パスワード
* MAIL_PASSWORD環境変数
---
* `from`
* string
* 送信者アドレス
* MAIL_FROM環境変数またはusername
---
* `subject`
* string
* メール件名
* "Dewy Notification"
---
* `tls`
* bool
* TLS暗号化の使用
* true
{% /table %}

### URL形式での設定

```bash
# URLパラメータで全設定を指定
dewy server --registry ghr://owner/repo \
  --notifier "smtp://smtp.gmail.com:587/admin@example.com?username=sender@gmail.com&password=app-password&from=sender@gmail.com&subject=Deploy+Notification"
```

### Gmail での設定例

```bash
# 環境変数を使用
export MAIL_USERNAME=sender@gmail.com
export MAIL_PASSWORD=your-app-password
export MAIL_FROM=sender@gmail.com

# Dewy実行
dewy server --registry ghr://owner/repo \
  --notifier "smtp://smtp.gmail.com:587/admin@example.com?subject=MyApp+Deploy"
```

{% callout type="important" %}
Gmailを使用する場合は、2要素認証を有効にしてアプリパスワードを生成する必要があります。
通常のGoogleアカウントパスワードでは認証できません。
{% /callout %}

## エラー通知の制限

Dewyは連続するエラー通知を制限して、スパムを防止します。

- **制限開始**: 連続3回のエラー後、通知を抑制
- **制限解除**: 正常な動作に戻ると自動的に制限を解除
- **制限中の動作**: ログは記録されるが通知は送信されない

```bash
# エラー通知制限の例
# 1回目: ✉️ Error notification sent
# 2回目: ✉️ Error notification sent  
# 3回目: ✉️ Error notification sent
# 4回目: 📝 Error logged (notification suppressed)
# 正常復旧: ✉️ Normal operation resumed, notification limit reset
```

## フック実行結果の通知

デプロイフック（Before/After Deploy Hook）の実行結果も通知されます：

### 成功時の通知例

```
🪝 Before Deploy Hook Success
Command: pg_dump mydb > backup.sql
Duration: 2.3s
Exit Code: 0
```

### 失敗時の通知例

```
❌ After Deploy Hook Failed
Command: systemctl reload nginx
Duration: 0.1s
Exit Code: 1
Error: Unit nginx.service not found
```

## 複数環境での通知設定

### 環境別チャンネル

```bash
# 本番環境
dewy server --registry ghr://owner/repo \
  --notifier "slack://prod-deploy?title=MyApp+Production"

# ステージング環境
dewy server --registry "ghr://owner/repo?pre-release=true" \
  --notifier "slack://staging-deploy?title=MyApp+Staging"

# 開発環境
dewy server --registry "ghr://owner/repo?pre-release=true" \
  --notifier "slack://dev-deploy?title=MyApp+Development"
```

### systemdでの環境設定

```systemd
# /etc/systemd/system/dewy-myapp-prod.service
[Unit]
Description=Dewy - MyApp Production

[Service]
Environment=SLACK_TOKEN=xoxb-prod-token
ExecStart=/usr/local/bin/dewy server \
  --registry ghr://owner/repo \
  --notifier "slack://prod-deploy?title=MyApp+Prod" \
  -- /opt/myapp/current/myapp

# /etc/systemd/system/dewy-myapp-staging.service
[Unit]
Description=Dewy - MyApp Staging

[Service]
Environment=SLACK_TOKEN=xoxb-staging-token
ExecStart=/usr/local/bin/dewy server \
  --registry "ghr://owner/repo?pre-release=true" \
  --notifier "slack://staging-deploy?title=MyApp+Staging" \
  -- /opt/myapp/current/myapp
```

## トラブルシューティング

Slack通知が届かない

1. **トークンの確認**
   ```bash
   # トークンのテスト
   curl -H "Authorization: Bearer $SLACK_TOKEN" \
     https://slack.com/api/auth.test
   ```
2. **権限の確認**
   - Bot Token Scopesで `channels:join` と `chat:write` が設定されているか
   - Appがワークスペースにインストールされているか
3. **チャンネル名の確認**
   ```bash
   # パブリックチャンネルの場合は # を除く
   # ❌ slack://#deployments
   # ✅ slack://deployments
   
   # プライベートチャンネルの場合は事前にBotを招待
   ```

メール通知が送信されない

1. **SMTP設定の確認**
   ```bash
   # SMTPサーバーへの接続テスト
   telnet smtp.gmail.com 587
   ```
2. **認証情報の確認**
   ```bash
   # 環境変数の確認
   echo $MAIL_USERNAME
   echo $MAIL_FROM
   # パスワードは表示しない
   ```
3. **TLS設定の確認**
   ```bash
   # TLSを無効にしてテスト（非推奨）
   dewy server --registry ghr://owner/repo \
     --notifier "smtp://smtp.example.com:25/admin@example.com?tls=false"
   ```

### デバッグ方法

```bash
# デバッグログで通知処理を確認
dewy server --registry ghr://owner/repo \
  --notifier slack://test-channel \
  --log-level debug

# 通知のみをテストする場合
dewy server --registry ghr://linyows/dewy \
  --notifier slack://test-channel \
  --log-level info
```

## 実際の運用例

### CI/CDパイプラインとの連携

```yaml
# GitHub Actions での通知設定
- name: Deploy to Production
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    SLACK_TOKEN: ${{ secrets.SLACK_TOKEN }}
  run: |
    dewy server \
      --registry ghr://${{ github.repository }} \
      --notifier "slack://deployments?title=${{ github.repository }}&url=https://github.com/${{ github.repository }}" \
      -- /opt/app/current/app
```

### 監視システムとの連携

```bash
# Datadogなどの監視システムとSlack通知の併用
dewy server --registry ghr://owner/repo \
  --notifier "slack://ops-alerts?title=MyApp+Production" \
  --after-deploy-hook "curl -X POST https://api.datadoghq.com/api/v1/events ..." \
  -- /opt/myapp/current/myapp
```

通知機能により、チーム全体でデプロイメント状況を共有し、問題の早期発見と対応が可能になります。適切な通知設定で、効率的な運用体制を構築してください。
