---
title: アーキテクチャ
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewyは、アプリケーションのSupervisor的な役割をし、Dewyがメインプロセスとなり、子プロセスとしてアプリケーションを起動させます。

## インターフェース

Dewyは、プラグガブルな抽象化として、Registry, Artifact, Cache, Notifierの４つのインターフェースから構成されています。

- Registry: バージョン管理（GitHub Releases, S3, GCS, gRPC）
- Artifact: バイナリ取得（対応するRegistry形式）
- Cache: ダウンロード済みファイル管理（File, Memory, Consul, Redis）
- Notifier: デプロイ通知（Slack, Mail）

## デプロイプロセス

以下はDewyのデプロイプロセスと構成を図にしたものです。

![Hi-level Architecture](https://github.com/linyows/dewy/blob/main/misc/dewy-architecture.svg?raw=true)

1. Registryは、指定したレジストリをポーリングし、アプリケーションの最新バージョンを検知する
1. Artifactは、指定したアーティファクトストアからアーティファクトをダウンロードし展開する
1. Cacheは、最新のバージョンとアーティファクトを保存する
1. Dewyは、新しいバージョンのアプリケーションの子プロセスを作り、リクエストを新しいアプリケーションでの処理を開始する
1. Registryは、何時どこで何をデプロイした情報をファイルとして保存する
1. Notifyerは、指定した通知先に通知を行う

このように、Dewyは中央のレジストリと通信し、いわゆるプル型のデプロイを実現します。
