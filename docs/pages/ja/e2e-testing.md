---
title: E2Eテスト
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewyは、すべてのデプロイメントモードとレジストリ統合の品質を保証するために、包括的なエンドツーエンド（E2E）テストを使用しています。このテストは、宣言的ワークフローツールである[Probe](https://github.com/linyows/probe)によって実行されます。

## テストの思想

E2Eテストは、以下の方法で実際のシナリオでDewyが正しく動作することを検証します：

- 実際のバイナリをビルド
- Dewyプロセスを実行し既存アプリのバージョンが動く
- レジストリに実際の新しいアプリのリリースを作成
- 新しいアプリのバージョンが動く
- デプロイ動作を検証しログにエラーがないことを確認

## テストカバレッジ

E2Eテストスイートは、コマンドとレジストリのすべての組み合わせをカバーしています：

| コマンド | レジストリ | 説明 |
|---------|----------|-------------|
| `server` | GitHub Releases | GitHub経由のバイナリサーバーデプロイ |
| `server` | AWS S3 | S3経由のバイナリサーバーデプロイ |
| `server` | Google Cloud Storage | GCS経由のバイナリサーバーデプロイ |
| `assets` | GitHub Releases | GitHub経由の静的アセットデプロイ |
| `assets` | AWS S3 | S3経由の静的アセットデプロイ |
| `assets` | Google Cloud Storage | GCS経由の静的アセットデプロイ |
| `container` | OCI Registry | GHCR経由のコンテナデプロイ |
| `container` (multi-port) | OCI Registry | マルチポートコンテナデプロイ |

## テストフローの可視化

E2Eテストのワークフローは、ProbeのDAG出力を使用して可視化できます：

```bash
probe --dag-mermaid testdata/e2e-test.yml
```

これにより、テスト実行フローを示すMermaid図が生成されます：

```mermaid
flowchart TD
    subgraph check["Check credentials"]
    end
    subgraph generate_version["Generate version"]
        generate_version_step0["Generate version string"]
    end
    subgraph build["Build dewy"]
        build_step0["Go build"]
    end
    subgraph create_release["Create new version"]
        create_release_step0["Create release"]
        create_release_step1["Verify release exists"]
    end
    subgraph job_4["Run server by Github-Releases registry"]
        job_4_step0["Server test"]
        job_4_step1["Start dewy"]
        job_4_step2["Wait for new version to start"]
        job_4_step3["Wait for stabilization"]
        job_4_step4["Stop dewy"]
        job_4_step5["Verify no error"]
        job_4_step6["Verify two symlinks creates"]
        job_4_step7["Verify starting two version"]
        job_4_step8["Verify starting new version"]
        job_4_step9["Show log"]
    end
    subgraph job_5["Run server by AWS S3 registry"]
        job_5_step0["Server test"]
        job_5_step1["Start dewy"]
        job_5_step2["Wait for new version to start"]
        job_5_step3["Wait for stabilization"]
        job_5_step4["Stop dewy"]
        job_5_step5["Verify no error"]
        job_5_step6["Verify two symlinks creates"]
        job_5_step7["Verify starting two version"]
        job_5_step8["Verify starting new version"]
        job_5_step9["Show log"]
    end
    subgraph job_6["Run server by GCloud Storage registry"]
        job_6_step0["Server test"]
        job_6_step1["Start dewy"]
        job_6_step2["Wait for new version to start"]
        job_6_step3["Wait for stabilization"]
        job_6_step4["Stop dewy"]
        job_6_step5["Verify no error"]
        job_6_step6["Verify two symlinks creates"]
        job_6_step7["Verify starting two version"]
        job_6_step8["Verify starting new version"]
        job_6_step9["Show log"]
    end
    subgraph job_7["Run assets by Github-Releases registry"]
        job_7_step0["Assets test"]
        job_7_step1["Start dewy"]
        job_7_step2["Wait for new version download"]
        job_7_step3["Wait for stabilization"]
        job_7_step4["Stop dewy"]
        job_7_step5["Verify no error"]
        job_7_step6["Verify two symlinks creates"]
        job_7_step7["Verify starting two version"]
        job_7_step8["Verify starting new version"]
        job_7_step9["Show log"]
    end
    subgraph job_8["Run assets by AWS S3 registry"]
        job_8_step0["Assets test"]
        job_8_step1["Start dewy"]
        job_8_step2["Wait for new version download"]
        job_8_step3["Wait for stabilization"]
        job_8_step4["Stop dewy"]
        job_8_step5["Verify no error"]
        job_8_step6["Verify two symlinks creates"]
        job_8_step7["Verify starting two version"]
        job_8_step8["Verify starting new version"]
        job_8_step9["Show log"]
    end
    subgraph job_9["Run assets by GCloud Storage registry"]
        job_9_step0["Assets test"]
        job_9_step1["Start dewy"]
        job_9_step2["Wait for new version download"]
        job_9_step3["Wait for stabilization"]
        job_9_step4["Stop dewy"]
        job_9_step5["Verify no error"]
        job_9_step6["Verify two symlinks creates"]
        job_9_step7["Verify starting two version"]
        job_9_step8["Verify starting new version"]
        job_9_step9["Show log"]
    end
    subgraph job_10["Run container by OCI registry"]
        job_10_step0["Container test"]
        job_10_step1["Start dewy"]
        job_10_step2["Wait for new version to start"]
        job_10_step3["Wait for stabilization"]
        job_10_step4["Stop dewy"]
        job_10_step5["Verify no error"]
        job_10_step6["Verify current 3 containers creates"]
        job_10_step7["Verify new 3 containers creates"]
        job_10_step8["Show log"]
    end
    subgraph job_11["Run container with multiple ports by OCI registry"]
        job_11_step0["Container multi-port test"]
        job_11_step1["Start dewy with multiple ports"]
        job_11_step2["Wait for new version to start"]
        job_11_step3["Wait for stabilization"]
        job_11_step4["Verify TCP proxy started on port1"]
        job_11_step5["Verify TCP proxy started on port2"]
        job_11_step6["Verify all TCP proxies started"]
        job_11_step7["Verify backends added to port1"]
        job_11_step8["Verify backends added to port2"]
        job_11_step9["Test connection to port1"]
        job_11_step10["Test connection to port2"]
        job_11_step11["Stop dewy"]
        job_11_step12["Verify no error"]
        job_11_step13["Verify new containers created"]
        job_11_step14["Show log"]
    end

    check --> generate_version
    generate_version --> build
    build --> create_release
    build --> job_4
    build --> job_5
    build --> job_6
    build --> job_7
    build --> job_8
    build --> job_9
    build --> job_10
    build --> job_11
```

## テスト構造

### 1. 認証情報の検証

テスト実行前に、必要な認証情報を検証します：

- `GITHUB_TOKEN` - GitHub ReleasesとGHCRアクセス用
- `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` - S3アクセス用
- `GOOGLE_APPLICATION_CREDENTIALS` - GCSアクセス用

### 2. ビルドフェーズ

各テストシナリオ用にDewyバイナリがビルドされます：

```yaml
- name: Go build for {{ vars.command }}-{{ vars.registry }}
  uses: shell
  with:
    cmd: go build -o ./testdata/{{ vars.command }}/{{ vars.registry }}/dewy ./cmd/dewy
```

### 3. リリース作成

一意のバージョンでGitHubにテストリリースが作成されます：

```yaml
- name: Create release
  uses: shell
  with:
    cmd: |
      gh release create {{ outputs.genver.version }} \
        --repo linyows/dewy-testapp \
        --title {{ outputs.genver.version }} \
        --notes "End-to-end Testing by Probe"
```

### 4. デプロイの検証

各テストジョブは以下を検証します：

1. **プロセス起動** - Dewyが正常に起動する
2. **バージョン検出** - 新バージョンが検出されデプロイされる
3. **アーティファクト処理** - ファイルが正しくダウンロード・展開される
4. **シンボリックリンク作成** - リリースのシンボリックリンクが作成される
5. **エラーフリー動作** - ログにエラーがない
6. **クリーンシャットダウン** - プロセスが正常に停止する

### コンテナ固有の検証

コンテナテストでは、追加のチェックが含まれます：

- 正しい数のレプリカが実行されている
- コンテナのヘルスチェックが合格している
- TCPプロキシが正しく機能している
- マルチポートルーティングが動作している
