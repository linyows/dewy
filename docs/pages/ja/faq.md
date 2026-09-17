---
title: よくある質問
---

# {% $markdoc.frontmatter.title %}

よくある質問をまとめました。

## Latestバージョンをレジストリから削除するとどうなりますか？

Dewyは削除後のLatestバージョンに変更します。リリースしたバージョンを削除したり上書きするのは望ましくありませんが、セキュリティの問題などやむを得ず削除するケースはあるかもしれません。

## オーディットログはどこにありますか？

オーディットログはアーティファクトがホストされてるところにテキストファイルのファイル名として保存されます。現状は検索性がないです。何かいい方法が思いついたら変更するでしょう。 オーディットとは別で通知としてOTELなどのオブザーバービリティプロダクトに送ることも必要かもしれません。

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