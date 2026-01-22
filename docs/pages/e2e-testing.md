---
title: End-to-End Testing
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy uses comprehensive end-to-end (E2E) testing to ensure quality across all deployment modes and registry integrations. This testing is powered by [Probe](https://github.com/linyows/probe), a declarative workflow tool.

## Testing Philosophy

E2E tests verify that Dewy works correctly in real-world scenarios by:

- Building actual binaries
- Running Dewy processes with existing app versions
- Creating real new app releases on registries
- Verifying new app versions start correctly
- Checking deployment behavior and ensuring no errors in logs

## Test Coverage

The E2E test suite covers all combinations of commands and registries:

| Command | Registry | Description |
|---------|----------|-------------|
| `server` | GitHub Releases | Binary server deployment via GitHub |
| `server` | AWS S3 | Binary server deployment via S3 |
| `server` | Google Cloud Storage | Binary server deployment via GCS |
| `assets` | GitHub Releases | Static assets deployment via GitHub |
| `assets` | AWS S3 | Static assets deployment via S3 |
| `assets` | Google Cloud Storage | Static assets deployment via GCS |
| `container` | OCI Registry | Container deployment via GHCR |
| `container` (multi-port) | OCI Registry | Multi-port container deployment |

## Test Flow Visualization

The E2E test workflow can be visualized using Probe's DAG output:

```bash
probe --dag-mermaid testdata/e2e-test.yml
```

This generates a Mermaid diagram showing the test execution flow:

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

## Test Structure

### 1. Credential Verification

Before running tests, required credentials are verified:

- `GITHUB_TOKEN` - For GitHub Releases and GHCR access
- `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` - For S3 access
- `GOOGLE_APPLICATION_CREDENTIALS` - For GCS access

### 2. Build Phase

Dewy binaries are built for each test scenario:

```yaml
- name: Go build for {{ vars.command }}-{{ vars.registry }}
  uses: shell
  with:
    cmd: go build -o ./testdata/{{ vars.command }}/{{ vars.registry }}/dewy ./cmd/dewy
```

### 3. Release Creation

A test release is created on GitHub with a unique version:

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

### 4. Deployment Verification

Each test job verifies:

1. **Process startup** - Dewy starts successfully
2. **Version detection** - New version is detected and deployed
3. **Artifact handling** - Files are downloaded and extracted correctly
4. **Symlink creation** - Release symlinks are created
5. **Error-free operation** - No errors in logs
6. **Clean shutdown** - Process stops gracefully

### Container-Specific Verification

For container tests, additional checks include:

- Correct number of replicas running
- Container health checks passing
- TCP proxy functioning correctly
- Multi-port routing working
