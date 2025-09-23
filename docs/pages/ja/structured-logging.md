---
title: 構造化ログ
---

# {% $markdoc.frontmatter.title %} {% #overview %}

dewyは構造化ログ機能を提供し、運用監視やトラブルシューティングに必要な情報を体系的に記録します。構造化ログとは、従来のテキストベースのログと異なり、キー・バリュー形式で情報を整理したログ形式です。この機能により、ログの検索・フィルタリング・集約が容易になり、自動化された監視システムとの連携も可能になります。

## ログフォーマットの選択

dewyでは、用途に応じて2つのログフォーマットから選択できます。環境や運用体制に応じて、最適なフォーマットを選択してください。

### text形式

text形式は人間が読みやすい形式で、開発環境やデバッグ作業に適しています。ログの内容を直接確認する必要がある場合や、問題の調査を手動で行う場合に有効です。

```bash
# テキスト形式での出力
dewy server --log-format text --log-level info --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

text形式の出力例：
```
time=2024-01-15T10:30:45.123Z level=INFO msg="Dewy started" version=v1.2.3 commit=abc1234 date=2024-01-15
time=2024-01-15T10:30:46.456Z level=INFO msg="Cached artifact" cache_key=v1.2.3--myapp_linux_amd64.tar.gz
```

このフォーマットは、コンソールでの表示や、シンプルなログファイルの確認に適しています。

### json形式

json形式は機械処理に適した形式で、本番環境でのログ集約システムとの連携に最適です。Elasticsearch、Logstash、Fluentd などのログ処理ツールとの親和性が高く、自動化された監視・分析が可能になります。

```bash
# JSON形式での出力
dewy server --log-format json --log-level info --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

json形式の出力例：
```json
{"time":"2024-01-15T10:30:45.123Z","level":"INFO","msg":"Dewy started","version":"v1.2.3","commit":"abc1234","date":"2024-01-15"}
{"time":"2024-01-15T10:30:46.456Z","level":"INFO","msg":"Cached artifact","cache_key":"v1.2.3--myapp_linux_amd64.tar.gz"}
```

このフォーマットにより、ログの各フィールドを個別にインデックス化し、高速な検索とフィルタリングが可能になります。

## コマンドライン設定

ログの設定は、コマンドライン引数を通じて制御できます。環境や用途に応じて、適切な組み合わせを選択してください。

### --log-level オプション

ログレベルは `--log-level` または `-l` オプションで指定します。指定可能な値は debug、info、warn、error で、大文字小文字は区別されません。

```bash
# ログレベルの指定
dewy server --log-level info --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
dewy server -l debug --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

デフォルトでは ERROR レベルが設定されています。

### --log-format オプション

ログフォーマットは `--log-format` または `-f` オプションで指定します。指定可能な値は text と json で、大文字小文字は区別されません。

```bash
# ログフォーマットの指定
dewy server --log-format json --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
dewy server -f text --registry ghr://myorg/app --port 8080 -- /opt/app/current/app
```

デフォルトでは text 形式が設定されています。

