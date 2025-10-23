---
title: インストール
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewyは単一のGoバイナリとして配布され、外部依存関係なしで動作します。以下の方法でインストールできます。

## 事前準備

Dewyを使用する前に、以下の要件を満たしていることを確認してください：

### システム要件

- **オペレーティングシステム**: Linux, macOS, Windows
- **アーキテクチャ**: amd64, arm64
- **シェル**: `/bin/sh` が利用可能であること（フック実行のため）
- **ネットワーク**: レジストリとアーティファクトストアへのアウトバウンド接続

### ファイルシステム要件

- **書き込み権限**: 作業ディレクトリに対する書き込み権限
- **シンボリックリンクサポート**: `current` ポインタ作成のため
- **一時ディレクトリ**: キャッシュストレージ用のアクセス権限
- **ディスク容量**: 7世代分のリリース + キャッシュ用の十分な容量（通常数百MB）

## インストール方法

### 1. プリビルドバイナリのダウンロード（推奨）

最新のリリースは [GitHub Releases](https://github.com/linyows/dewy/releases) からダウンロードできます。

#### 手動ダウンロード

```bash
# 最新リリースのURLを確認
LATEST_VERSION=$(curl -s https://api.github.com/repos/linyows/dewy/releases/latest | grep '"tag_name"' | cut -d '"' -f 4)

# アーキテクチャに応じたバイナリをダウンロード（Linux amd64の例）
wget https://github.com/linyows/dewy/releases/download/${LATEST_VERSION}/dewy_linux_x86_64.tar.gz

# 展開とインストール
tar -xzf dewy_linux_x86_64.tar.gz
sudo mv dewy /usr/local/bin/
chmod +x /usr/local/bin/dewy
```

#### プラットフォーム別バイナリ

| OS    | Architecture | Filename                    |
| ---   | ---          | ---                         |
| Linux | amd64        | `dewy_linux_x86_64.tar.gz`  |
| Linux | arm64        | `dewy_linux_arm64.tar.gz`   |
| macOS | amd64        | `dewy_darwin_x86_64.tar.gz` |
| macOS | arm64        | `dewy_darwin_arm64.tar.gz`  |

### 2. ソースコードからビルド

Go 1.21以上が必要です。

```bash
# リポジトリをクローン
git clone https://github.com/linyows/dewy.git
cd dewy

# 依存関係の取得
go mod download

# ビルド
go build -o dewy

# システムディレクトリにインストール
sudo mv dewy /usr/local/bin/
```

#### 開発版のビルド

```bash
# 最新のmainブランチから直接インストール
go install github.com/linyows/dewy@latest
```

### 3. プロビジョニングツール

本番環境での大規模展開には、以下のプロビジョニングツールを使用できます。

#### Chef

Chef Cookbookを使用したインストール：

```bash
# Cookbookを取得
# https://github.com/linyows/dewy-cookbook を参照
```

Chef Recipeの例：

```ruby
# dewyをインストール
dewy 'myapp' do
  registry 'ghr://myorg/myapp'
  notifier 'slack://deployments?title=myapp'
  ports ['8000']
  log_level 'info'
  action :install
end

# systemdサービスとして設定
systemd_unit 'dewy-myapp.service' do
  content <<~EOS
    [Unit]
    Description=Dewy - myapp deployment manager
    After=network.target

    [Service]
    Type=simple
    User=deploy
    ExecStart=/usr/local/bin/dewy server --registry ghr://myorg/myapp \\
              --notifier slack://deployments?title=myapp \\
              --port 8000 --log-level info \\
              -- /opt/myapp/current/myapp
    Restart=always
    RestartSec=5

    [Install]
    WantedBy=multi-user.target
  EOS
  action [:create, :enable, :start]
end
```

#### Puppet

Puppet Moduleを使用したインストール：

```bash
# Puppet Moduleを取得
# https://github.com/takumakume/puppet-dewy を参照
```

Puppetマニフェストの例：

```puppet
# dewyをインストール
class { 'dewy':
  version => '1.2.3',
  install_method => 'binary',
}

# アプリケーション設定
dewy::app { 'myapp':
  registry => 'ghr://myorg/myapp',
  notifier => 'slack://deployments?title=myapp',
  ports    => ['8000'],
  log_level => 'info',
  command  => '/opt/myapp/current/myapp',
  user     => 'deploy',
  group    => 'deploy',
}
```

## インストール確認

インストールが正常に完了したか確認します：

```bash
# バージョン確認
dewy --version

# ヘルプ表示
dewy --help

# 基本的な動作確認
dewy server --registry ghr://linyows/dewy --help
```

## 次のステップ

インストールが完了したら、以下のドキュメントを参照して設定を開始してください：

- [はじめに](../getting-started/)
- [アーキテクチャ](../architecture/)
- [よくある質問](../faq/)

## トラブルシューティング

### 一般的な問題

#### 権限エラー

```bash
# /usr/local/binに書き込み権限がない場合
sudo chmod 755 /usr/local/bin
sudo chown root:root /usr/local/bin/dewy

# または、ユーザーディレクトリにインストール
mkdir -p ~/bin
mv dewy ~/bin/
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

#### シンボリックリンクエラー

```bash
# ファイルシステムがシンボリックリンクをサポートしているか確認
ln -s /tmp/test /tmp/testlink && rm /tmp/testlink || echo "Symlinks not supported"
```

#### ネットワーク接続の問題

```bash
# GitHub APIへの接続確認
curl -s https://api.github.com/repos/linyows/dewy/releases/latest

# プロキシ環境の場合
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

### ログとデバッグ

```bash
# デバッグログを有効にして実行
dewy server --registry ghr://owner/repo --log-level debug --log-format json

# システムログの確認（systemd使用時）
journalctl -u dewy-myapp.service -f
```

問題が解決しない場合は、[GitHub Issues](https://github.com/linyows/dewy/issues) でサポートを求めてください。
