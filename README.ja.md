<p align="right"><a href="https://github.com/linyows/dewy/blob/main/README.md">English</a> | 日本語</p>

<p align="center">
  <a href="https://dewy.linyo.ws">
    <br><br><br><br><br><br>
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://github.com/linyows/dewy/blob/main/misc/dewy-dark-bg.svg?raw=true">
      <img alt="Dewy" src="https://github.com/linyows/dewy/blob/main/misc/dewy.svg?raw=true" width="240">
    </picture>
    <br><br><br><br><br><br>
  </a>
</p>

<p align="center">
  <strong>Dewy</strong> enables declarative deployment of applications in non-Kubernetes environments.
</p>

<p align="center">
  <a href="https://github.com/linyows/dewy/actions/workflows/build.yml">
    <img alt="GitHub Workflow Status" src="https://img.shields.io/github/actions/workflow/status/linyows/dewy/build.yml?branch=main&style=for-the-badge&labelColor=000000">
  </a>
  <a href="https://github.com/linyows/dewy/releases">
    <img src="http://img.shields.io/github/release/linyows/dewy.svg?style=for-the-badge&labelColor=000000" alt="GitHub Release">
  </a>
  <a href="http://godoc.org/github.com/linyows/dewy">
    <img src="http://img.shields.io/badge/go-documentation-blue.svg?style=for-the-badge&labelColor=000000" alt="Go Documentation">
  </a>
</p>

Dewyは、主にGoで作られたアプリケーションを宣言的にデプロイするソフトウェアです。
Dewyは、アプリケーションのSupervisor的な役割をし、Dewyがメインプロセスとなり、子プロセスとしてアプリケーションを起動させます。
Dewyのスケジューラーは、指定する「レジストリ」をポーリングし、セマンティックバージョニングで管理された最新のバージョンを検知すると、指定する「アーティファクト」ストアからデプロイを行います。
Dewyは、いわゆるプル型のデプロイを実現します。Dewyは、レジストリ、アーティファクトストア、キャッシュストア、通知の４つのインターフェースから構成されています。
以下はDewyのデプロイプロセスと構成を図にしたものです。

<p align="center">
  <img alt="Dewyのデプロイプロセスとアーキテクチャ" src="https://github.com/linyows/dewy/blob/main/misc/dewy-architecture.svg?raw=true" width="640"/>
</p>

主な機能
--

- プル型デプロイメント
- グレースフルリスタート
- 選択可能なレジストリとアーティファクトストア
- デプロイ状況の通知
- オーディットログ

使いかた
--

次のServerコマンドは、registryにgithub releasesを使い、8000番ポートでサーバ起動し、ログレベルをinfoに設定し、slackに通知する例です。

```sh
$ export GITHUB_TOKEN=****.....
$ export SLACK_TOKEN=****.....
$ dewy server --registry ghr://linyows/dewy-testapp -p 8000 -l info -- /opt/dewy/current/testapp
```

Github APIとSlack APIを使うので、それぞれ環境変数をセットしています。
レジストリと通知の指定はurlを模擬した構成になっています。urlのschemeにあたる箇所はレジストリや通知の名前です。

```sh
# github releasesレジストリの場合：
--registry ghr://<owner-name>/<repo-name>

# aws s3レジストリの場合：
--registry s3://<bucket-name>/<object-prefix>
```

コマンド
--

Dewyには、ServerとAssetsコマンドがあります。
ServerはServer Application用でApplicationのプロセス管理を行い、Applicationのバージョンを最新に維持します。
Assetsはhtmlやcssやjsなど、静的ファイルのバージョンを最新に維持します。

インターフェース
--

Dewyにはいくつかのインターフェースがあり、それぞれ選択可能な実装を持っています。各インターフェースの説明をします。

Interface | Description
---       | ---
Registry  | レジストリは、アプリケーションやファイルのバージョンを管理するインターフェースです。レジストリの実装には、Github ReleasesとAWS S3とGRPCがあります。GRPCは、インターフェースを満たすサーバを自作することができ、既存APIをレジストリにすることができます。
Artifact  | アーティファクトは、アプリケーションやファイルそのものを管理するインターフェースです。アーティファクトの実装には、Github ReleaseとAWS S3とGoogle Cloud Storageがあります。
Cache     | キャッシュは、現在のバージョンやアーティファクトをDewyが保持するためのインターフェースです。キャッシュの実装には、ファイルシステムとメモリとhashicorp consulとredisがあります。
Notify    | 通知は、デプロイの状態を通知するインターフェースです。通知の実装は、Slackがあります。

各インターフェースで必要な実装があればissueを作ってください。

セマンティックバージョニング
--

Dewyは、セマンティックバージョニングに基づいてバージョンのアーティファクトの新しい古いを判別しています。
そのため、ソフトウェアのバージョンをセマンティックバージョニングで管理しなければなりません。

```txt
# Pre release versions：
v1.2.3-rc
v1.2.3-beta.2
```

ステージング
--

セマンティックバージョニングには、プレリリースという考え方があります。バージョンに対してハイフンをつけてsuffixを付加したものが
プレリリースバージョンになります。ステージング環境では、registryのオプションに `pre-release=true`を追加することで、プレリリースバージョンがデプロイされるようになります。

背景
--

Goはコードを各環境に合わせたひとつのバイナリにコンパイルすることができます。
Kubernetesのようなオーケストレーターのある分散システムでは、Goで作られたアプリケーションのデプロイに困ることはないでしょう。
一方で、コンテナではない単一の物理ホストや仮想マシン環境において、Goのバイナリをどうやってデプロイするかの明確な答えはないように思います。
手元からscpやrsyncするshellを書いて使うのか、サーバ構成管理のansibleを使うのか、rubyのcapistranoを使うのか、方法は色々あります。
しかし、複数人のチームで誰がどこにデプロイしたといったオーディットログや情報共有を考えると、そのようなユースケースにマッチするツールがない気がします。

作者
--

[@linyows](https://github.com/linyows)
