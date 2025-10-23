---
title: 使ってみよう
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewyを使って実際にアプリケーションをデプロイしてみましょう。この記事では、基本的な使い方から実際のデプロイメントまでを順を追って説明します。

## 前提条件

- Dewyがインストールされていること（[インストールガイド](/ja/installation)参照）
- デプロイしたいGoアプリケーションがGitHub Releasesで公開されていること
- 必要な環境変数が設定されていること

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

## 次のステップ

さらに詳しく知りたい場合は、以下のドキュメントを参照してください：

- [アーキテクチャ](../architecture/)
- [Dewy CLIリファレンス](../reference/)
- [よくある質問](../faq/)
