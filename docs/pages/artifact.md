---
title: Artifact
description: |
  Artifacts are Dewy components that manage actual application binaries and files.
  They download files corresponding to versions identified in the registry and prepare them for deployment.
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## Artifact Types

Dewy supports the following artifact types:

- **GitHub Releases** (`ghr://`): GitHub release attachments
- **AWS S3** (`s3://`): S3 object storage files
- **Google Cloud Storage** (`gs://`): GCS object storage files

Artifact types are automatically linked with registry types.

## File Formats

### Supported Archive Formats

Dewy supports the following archive formats:

- **tar.gz / tgz**: Most common format
- **tar**: Uncompressed tar
- **zip**: Format commonly used in Windows environments

### Archive Structure

It is recommended to create artifacts with the following structure:

```
myapp_linux_amd64.tar.gz
├── myapp                 # Executable binary
├── config/
│   └── app.conf         # Configuration file
├── static/
│   ├── css/
│   └── js/
└── README.md
```

## File Naming Conventions

When artifact names are not explicitly specified, Dewy automatically selects files using the following patterns:

```bash
# Recommended pattern
<app-name>_<os>_<arch>.<ext>

# Examples
myapp_linux_amd64.tar.gz
myapp_darwin_arm64.tar.gz
myapp_windows_amd64.zip
```

### OS Identifiers

{% table %}
* OS
* Identifier
* Example
---
* Linux
* `linux`
* `myapp_linux_amd64.tar.gz`
---
* macOS
* `darwin`, `macos`
* `myapp_darwin_arm64.tar.gz`
---
* Windows
* `windows`, `win`
* `myapp_windows_amd64.zip`
{% /table %}

### Architecture Identifiers

{% table %}
* Architecture
* Identifier
* Example
---
* x86_64
* `amd64`, `x86_64`
* `myapp_linux_amd64.tar.gz`
---
* ARM64
* `arm64`, `aarch64`
* `myapp_darwin_arm64.tar.gz`
---
* ARM32
* `arm`, `armv7`
* `myapp_linux_arm.tar.gz`
{% /table %}

## GitHub Releases Artifacts

Basic configuration

```bash
# Registry URL
ghr://owner/repo

# Auto-selected example (for Linux amd64 environment)
myapp_linux_amd64.tar.gz
```

### Release Creation Example

Release creation and artifact attachment with GitHub Actions:

```yaml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        include:
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
          - os: macos-latest
            goos: darwin
            goarch: arm64
          - os: windows-latest
            goos: windows
            goarch: amd64

    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: |
          go build -o myapp
          tar -czf myapp_${{ matrix.goos }}_${{ matrix.goarch }}.tar.gz myapp

      - name: Upload to release
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ steps.create_release.outputs.upload_url }}
          asset_path: ./myapp_${{ matrix.goos }}_${{ matrix.goarch }}.tar.gz
          asset_name: myapp_${{ matrix.goos }}_${{ matrix.goarch }}.tar.gz
          asset_content_type: application/gzip
```

### Specifying Specific Artifacts

```bash
# Specify a specific artifact when multiple artifacts exist
dewy server --registry "ghr://owner/repo?artifact=myapp-server.tar.gz"
```

## AWS S3 Artifacts

Directory structure

```
s3://my-bucket/releases/myapp/
├── v1.2.3/
│   ├── myapp_linux_amd64.tar.gz
│   ├── myapp_linux_arm64.tar.gz
│   ├── myapp_darwin_arm64.tar.gz
│   └── myapp_windows_amd64.zip
├── v1.2.2/
│   ├── myapp_linux_amd64.tar.gz
│   └── myapp_darwin_arm64.tar.gz
└── v1.2.1/
    └── myapp_linux_amd64.tar.gz
```

### Upload Example

```bash
# Upload using AWS CLI
aws s3 cp myapp_linux_amd64.tar.gz \
  s3://my-bucket/releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz

# Automatic upload with GitHub Actions
- name: Upload to S3
  env:
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
  run: |
    aws s3 cp myapp_linux_amd64.tar.gz \
      s3://my-bucket/releases/myapp/${GITHUB_REF_NAME}/
```

## Google Cloud Storage Artifacts

Directory structure

```
gs://my-releases/myapp/
├── v1.2.3/
│   ├── myapp_linux_amd64.tar.gz
│   ├── myapp_linux_arm64.tar.gz
│   └── myapp_darwin_arm64.tar.gz
└── v1.2.2/
    └── myapp_linux_amd64.tar.gz
```

### Upload Example

```bash
# Upload using gsutil
gsutil cp myapp_linux_amd64.tar.gz \
  gs://my-releases/myapp/v1.2.3/

# Automatic upload with GitHub Actions
- name: Upload to GCS
  uses: google-github-actions/setup-gcloud@v1
  with:
    service_account_key: ${{ secrets.GCP_SA_KEY }}

- name: Upload artifact
  run: |
    gsutil cp myapp_linux_amd64.tar.gz \
      gs://my-releases/myapp/${GITHUB_REF_NAME}/
```

## Artifact Verification

To ensure artifact integrity, it is recommended to place checksum files alongside artifacts.

```bash
# Generate SHA256 checksum
sha256sum myapp_linux_amd64.tar.gz > myapp_linux_amd64.tar.gz.sha256

# Upload (GitHub Releases example)
# - myapp_linux_amd64.tar.gz
# - myapp_linux_amd64.tar.gz.sha256
```

In security-focused environments, GPG signatures can also be provided.

```bash
# Generate GPG signature
gpg --detach-sign --armor myapp_linux_amd64.tar.gz

# Result
# - myapp_linux_amd64.tar.gz
# - myapp_linux_amd64.tar.gz.asc
```

## Troubleshooting

### Artifact Not Found

Check naming conventions:

```bash
# Correct example
myapp_linux_amd64.tar.gz

# Unrecognized examples
myapp-1.2.3.tar.gz
linux-binary.tar.gz
```

Check file existence:

```bash
# Check GitHub Releases
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/owner/repo/releases/latest"
```

### Extraction Errors

Check archive format:

```bash
# Check file format
file myapp_linux_amd64.tar.gz

# Manual extraction test
tar -tzf myapp_linux_amd64.tar.gz
```

Check permissions:

```bash
# Set execution permissions
chmod +x myapp
```

### Debugging Methods

```bash
# Debug artifact download
dewy server --registry ghr://owner/repo --log-level debug

# Test with specific artifact specification
dewy server --registry "ghr://owner/repo?artifact=specific-file.tar.gz"
```

Artifact management is a crucial element in Dewy's deployment process. Proper naming conventions and structured file placement enable automated deployment.