---
title: よくある質問
---

# {% $markdoc.frontmatter.title %}

よくある質問をまとめました。

## Latestバージョンをレジストリから削除するとどうなりますか？

Dewyは削除後のLatestバージョンに変更します。リリースしたバージョンを削除したり上書きするのは望ましくありませんが、セキュリティの問題などやむを得ず削除するケースはあるかもしれません。

## オーディットログはどこにありますか？

dewyはデプロイのたびに、デプロイ元のレジストリへ記録を残します。記録にはホスト名、コマンド、時刻が含まれます（例: `shipped_to_web-1_server_at_20261122T083000.123456789Z.txt`）。

| レジストリ | 記録の保存先 |
|---|---|
| GitHub Releases | この名前のテキストファイルを、デプロイしたリリースのアセットとしてアップロード |
| S3 / GCS | `<prefix><tag>/`の下に、この名前の空のオブジェクトを作成 |
| gRPC | `Report`の呼び出しでgRPCサーバーに送信。保存方法はサーバーが決める |
| OCI | 記録しない |

記録は対象のリリースと同じ場所に残るため、あるバージョンを動かしているホストはそのリリースから一覧できます。リリースをまたいでデプロイの履歴を一覧するには、すべてのリリースのアセットやオブジェクトを読む必要があります。履歴を自前のデータベースに保存したい場合は、すべての記録を受け取る[gRPCレジストリ](/ja/registry#grpc)を使ってください。GitHub Releasesに記録するには、リポジトリのContentsへの書き込み権限が必要です。[レジストリ](/ja/registry#github-releases)を参照してください。

[テレメトリ](/ja/telemetry)はこの記録の代わりにはなりません。dewyがエクスポートするOpenTelemetryのメトリクス（`dewy.deployments.total`や`dewy.deployments.rollbacks.total`など）は、コマンドごとのデプロイ回数を数えるもので、ダッシュボードやアラートに向いていますが、個々のデプロイのバージョンやホストは識別できません。デプロイを1件ずつイベントとしてレジストリの外で受け取るには、[通知](/ja/notifier)を使うか、[JSON形式のログ](/ja/structured-logging)をログ基盤で収集してください。

## 複数Dewyからのポーリングによってレジストリのレートリミットにかかるのはどう対処できますか？

S3またはGoogle Cloud Storageの同じprefixを複数のDewyで共有し、cache URLに `registry-ttl` を付けてください。

```sh
dewy server --registry ghr://owner/repo \
  --cache 's3://ap-northeast-1/mybucket/myapp?registry-ttl=30s' \
  -- /opt/myapp/current/myapp
```

TTLウィンドウあたり1台だけが上流registryをpollし、残りは共有キャッシュからレスポンスを読みます。rate limitを消費するのはpollingです。新しいリリースの有無にかかわらず、インスタンスごとにinterval間隔で発生するためです。したがって効くのはこの設定です。[Registry result cache](/ja/cache#registry-result-cache)を参照してください。

`--interval` でポーリング間隔を長くするのも有効で、両者は併用できます。

## 同一ホストで複数のDewyを実行するにはどうすればいいでしょうか？

Dewyはカレントワーキングディレクトリ（cwd）にキャッシュファイルを作成します。同一ホストで複数のDewyインスタンスを実行するには、それぞれ異なるディレクトリから実行してください。これにより、各インスタンスが独自のキャッシュと状態ファイルを維持し、競合を避けることができます。

例：

```bash
# 1つ目のインスタンス
mkdir -p /opt/app1 && cd /opt/app1
dewy server --registry ghr://owner/repo1 --port 8001 -- /opt/app1/current/app

# 2つ目のインスタンス
mkdir -p /opt/app2 && cd /opt/app2
dewy server --registry ghr://owner/repo2 --port 8002 -- /opt/app2/current/app
```

## 次のステップ

さらに詳しく知りたい場合は、以下のドキュメントを参照してください：

- [使ってみよう](../getting-started)
- [アーキテクチャ](../architecture)
- [コントリビューティング](../contributing)