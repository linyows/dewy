---
title: Registry
description: |
  Registry is the core component of Dewy that handles application and file version management.
  Dewy automatically detects the latest version based on semantic versioning and achieves continuous deployment.
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## Supported Registries

Dewy supports the following registry types:

- **GitHub Releases** (`ghr://`): GitHub release functionality
- **AWS S3** (`s3://`): Amazon S3 storage
- **Google Cloud Storage** (`gs://`): Google Cloud storage
- **OCI Registry** (`img://`): OCI-compliant container registries (Docker Hub, GHCR, GCR, ECR, etc.)
- **gRPC** (`grpc://`): Custom gRPC server

## Common Options

Common options available for all registries:

{% table %}
* Option
* Type
* Description
---
* `pre-release`
* bool
* Whether to include pre-release versions
---
* `artifact`
* string
* Explicitly specify artifact names that are not automatically selected
{% /table %}

## GitHub Releases

The most common method for using GitHub releases as a registry.

### Basic Configuration

```bash
# Basic format
ghr://<owner>/<repo>

# Example
dewy server --registry ghr://linyows/myapp -- /opt/myapp/current/myapp
```

### Authentication

Dewy supports two authentication methods for GitHub: Personal Access Token (PAT) and GitHub App.

#### Personal Access Token (PAT)

The simplest authentication method. Set via environment variables:

```bash
# GH_TOKEN takes precedence (recommended to avoid GitHub Actions auto-override)
export GH_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx

# Or use GITHUB_TOKEN
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

{% callout type="note" %}
PATs have a maximum validity of 1 year. For long-term operations, GitHub App authentication is recommended.
{% /callout %}

#### GitHub App Authentication (Recommended for Production)

GitHub App authentication is recommended for production environments as it offers:

- **Automatic token refresh**: No manual token rotation required
- **Fine-grained permissions**: Only request necessary permissions
- **Better security**: No long-lived tokens to manage
- **Audit logs**: Better tracking of API usage

**Environment Variables:**

{% table %}
* Variable
* Description
* Required
---
* `GITHUB_APP_ID`
* GitHub App ID
* Yes
---
* `GITHUB_APP_INSTALLATION_ID`
* Installation ID
* Yes
---
* `GITHUB_APP_PRIVATE_KEY`
* PEM format private key (direct content)
* Yes*
---
* `GITHUB_APP_PRIVATE_KEY_PATH`
* Path to private key file
* Yes*
{% /table %}

*Either `GITHUB_APP_PRIVATE_KEY` or `GITHUB_APP_PRIVATE_KEY_PATH` is required.

**Example:**

```bash
export GITHUB_APP_ID=123456
export GITHUB_APP_INSTALLATION_ID=789012
export GITHUB_APP_PRIVATE_KEY_PATH=/path/to/private-key.pem

dewy server --registry ghr://owner/repo -- /opt/app/current/app
```

#### Authentication Priority

When multiple authentication methods are configured, Dewy uses the following priority:

1. **GitHub App** (if `GITHUB_APP_ID` is set)
2. **PAT** (`GH_TOKEN` > `GITHUB_TOKEN`)

#### GitHub Enterprise Server

For GitHub Enterprise Server, set the API endpoint:

```bash
export GITHUB_API_URL=https://github.example.com/api/v3/
# or
export GITHUB_ENDPOINT=https://github.example.com/api/v3/
```

### Creating a GitHub App

To use GitHub App authentication, you need to create a GitHub App:

#### 1. Create the App

1. Go to **Settings** → **Developer settings** → **GitHub Apps** → **New GitHub App**
2. Fill in basic settings:
   - **GitHub App name**: e.g., `dewy-deployer`
   - **Homepage URL**: Your repository URL or documentation
3. Configure Webhook:
   - **Active**: Uncheck (Dewy uses polling, not webhooks)
4. Set Permissions:
   - **Repository permissions**:
     - **Contents**: Read & Write (required for releases, assets, and deployment audit records)
5. Choose installation scope:
   - **Only on this account** or **Any account**

{% callout type="note" %}
Write permission is required for the deployment audit feature, which records shipping information as release assets.
{% /callout %}

#### 2. Generate Private Key

1. After creating the App, go to the App settings page
2. Scroll to **Private keys** section
3. Click **Generate a private key**
4. Save the downloaded `.pem` file securely

#### 3. Install the App

1. Go to App settings → **Install App**
2. Select the organization/user account
3. Choose **Only select repositories** and select target repositories
4. Click **Install**

#### 4. Get Required Values

- **App ID**: Found on the App settings page under "App ID"
- **Installation ID**: After installation, check the URL:
  - `https://github.com/settings/installations/XXXXXX`
  - The `XXXXXX` part is your Installation ID

### Examples with Options

```bash
# Include pre-release versions (staging environment)
dewy server --registry "ghr://linyows/myapp?pre-release=true"

# Specify specific artifact
dewy server --registry "ghr://linyows/myapp?artifact=myapp-server.tar.gz"

# Use both options
dewy server --registry "ghr://linyows/myapp?pre-release=true&artifact=myapp-server.tar.gz"
```

### Automatic Artifact Selection

When artifact names are not specified, Dewy automatically selects using the following rules:

1. File names containing current OS (`linux`, `darwin`, `windows`)
2. File names containing current architecture (`amd64`, `arm64`, etc.)
3. Select the first matching artifact

Example: In Linux amd64 environment, `myapp_linux_amd64.tar.gz` is automatically selected.

{% callout type="important" %}
For newly created releases, there is a 30-minute grace period considering CI/CD artifact build time.
During this period, "artifact not found" errors are not notified.
{% /callout %}

## AWS S3

S3-compatible storage can be used as a registry.

### Basic Configuration

```bash
# Basic format
s3://<region>/<bucket>/<path-prefix>

# Example
dewy server --registry s3://ap-northeast-1/my-releases/myapp
```

### Environment Variables

```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Optional: Endpoint URL (for S3-compatible services)
export AWS_ENDPOINT_URL=https://s3.isk01.sakurastorage.jp
```

### Object Path Rules

Objects in S3 must be arranged in the following structure:

```
<path-prefix>/<semver>/<artifact>
```

Actual example:

```
my-releases/myapp/v1.2.4/myapp_linux_amd64.tar.gz
my-releases/myapp/v1.2.4/myapp_linux_arm64.tar.gz
my-releases/myapp/v1.2.4/myapp_darwin_arm64.tar.gz
my-releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz
my-releases/myapp/v1.2.3/myapp_linux_arm64.tar.gz
my-releases/myapp/v1.2.3/myapp_darwin_arm64.tar.gz
```

## Google Cloud Storage

Google Cloud Storage can be used as a registry.

### Basic Configuration

```bash
# Basic format
gs://<bucket>/<path-prefix>

# Example
dewy server --registry gs://my-releases-bucket/myapp
```

### Environment Variables

```bash
# Use service account key
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

Google Cloud SDK credentials or Workload Identity can also be used.

### Object Path Rules

Arranged in the same structure as S3:

```
myapp/v1.2.4/myapp_linux_amd64.tar.gz
myapp/v1.2.4/myapp_darwin_arm64.tar.gz
myapp/v1.2.3/myapp_linux_amd64.tar.gz
```

## OCI Registry

OCI-compliant container registries can be used for container image deployment with the `dewy container` command.

{% callout type="caution" %}
Audit tracking is not supported for OCI registries.
{% /callout %}

### Supported Registries

Dewy supports all OCI Distribution Specification-compliant registries:

- **GitHub Container Registry** (ghcr.io)
- **Docker Hub** (docker.io)
- **Google Artifact Registry** (gcr.io, us-docker.pkg.dev)
- **Amazon Elastic Container Registry** (ECR)
- **Azure Container Registry** (azurecr.io)
- **Private/self-hosted registries** (Harbor, Nexus, etc.)

### Basic Configuration

```bash
# GitHub Container Registry
img://ghcr.io/<owner>/<repository>

# Docker Hub
img://docker.io/<owner>/<repository>
# or shorter format
img://<owner>/<repository>

# Google Artifact Registry
img://gcr.io/<project-id>/<repository>

# Private registry
img://registry.example.com/<repository>
```

### Authentication

Dewy uses Docker's authentication system. Authenticate using `docker login`:

```bash
# Login to registry (credentials saved to ~/.docker/config.json)
docker login ghcr.io
docker login docker.io

# Dewy will automatically use these credentials
dewy container --registry img://ghcr.io/myorg/myapp
```

### Examples with Options

```bash
# Basic usage
dewy container --registry img://ghcr.io/myorg/myapp

# Include pre-release versions
dewy container --registry "img://ghcr.io/myorg/myapp?pre-release=true"

# With container options
dewy container --registry img://ghcr.io/myorg/myapp \
  --port 8000:8080 \
  --health-path /health
```

### Registry-Specific Examples

#### GitHub Container Registry (GHCR)

```bash
# Public image
dewy container --registry img://ghcr.io/owner/app

# Private image (requires authentication)
docker login ghcr.io
dewy container --registry img://ghcr.io/owner/private-app
```

#### Docker Hub

```bash
# Official image (library namespace)
dewy container --registry img://docker.io/library/nginx

# User image
dewy container --registry img://docker.io/myuser/myapp

# Shorter format (docker.io is default)
dewy container --registry img://myuser/myapp
```

#### Google Artifact Registry

```bash
# Authenticate with gcloud
gcloud auth configure-docker gcr.io

# Deploy image
dewy container --registry img://gcr.io/my-project/myapp
```

#### AWS ECR

```bash
# Login to ECR
aws ecr get-login-password --region ap-northeast-1 | \
  docker login --username AWS --password-stdin \
  123456789.dkr.ecr.ap-northeast-1.amazonaws.com

# Deploy image
dewy container --registry img://123456789.dkr.ecr.ap-northeast-1.amazonaws.com/myapp
```

### Tag and Version Selection

Dewy automatically selects container image tags based on semantic versioning:

```bash
# Tags in registry:
# - v1.2.3
# - v1.2.2
# - v1.2.3-beta.1
# - latest

# Production (stable versions only, selects v1.2.3)
dewy container --registry img://ghcr.io/myorg/myapp

# Staging (including pre-release, selects v1.2.3-beta.1 if newer)
dewy container --registry "img://ghcr.io/myorg/myapp?pre-release=true"
```

### Multi-Architecture Support

Dewy automatically selects the appropriate architecture using OCI Image Index (manifest list):

```bash
# Automatically pulls amd64, arm64, or other architectures
# based on the host system
dewy container --registry img://ghcr.io/myorg/myapp
```

### Rolling Update Deployment Workflow

When using OCI registry with `dewy container` command:

1. Dewy polls the registry for new tags at the specified interval
2. New semver-compliant tags are detected automatically
3. New container image is pulled
4. Health check is performed on new container (if configured)
6. Old container is drained and removed

```bash
# Complete example with all features
dewy container \
  --registry img://ghcr.io/myorg/myapp \
  --interval 300 \
  --port 8000:8080 \
  --health-path /health \
  --health-timeout 30 \
  --drain-time 30 \
  --log-level info
```

{% callout type="important" %}
OCI registries are only used with the `dewy container` command for container deployments.
For binary deployments, use GitHub Releases, S3, or GCS registries.
{% /callout %}

## gRPC

Custom gRPC servers can be used as registries.

### Basic Configuration

```bash
# Basic format
grpc://<server-host>

# Example
dewy server --registry grpc://registry.example.com:9000

# Without TLS
dewy server --registry "grpc://localhost:9000?no-tls=true"
```

### Features

- gRPC server dynamically provides artifact URLs
- `pre-release` and `artifact` options cannot be used
- Flexible control with custom logic is possible

## Semantic Versioning

Dewy performs version management compliant with semantic versioning (semver).

### Supported Formats

```bash
# Standard versions
v1.2.3
1.2.3

# Pre-release versions
v1.2.3-rc
v1.2.3-beta.2
v1.2.3-alpha.1
```

### Using Pre-release Versions

```bash
# Production environment (stable versions only)
dewy server --registry ghr://myorg/myapp

# Staging environment (including pre-release versions)
dewy server --registry "ghr://myorg/myapp?pre-release=true"
```

## CI/CD Pipeline Usage

```bash
# GitHub Actions
- name: Deploy with Dewy
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    dewy server --registry ghr://${{ github.repository }} \
      --log-level info --port 8080 -- /opt/app/current/app
```

## Multi-stage Deployment

```bash
# Staging environment
ENVIRONMENT=staging dewy server \
  --registry "ghr://myorg/myapp?pre-release=true" \
  --notifier "slack://staging-deploy?title=myapp-staging"

# Production environment
ENVIRONMENT=production dewy server \
  --registry "ghr://myorg/myapp" \
  --notifier "slack://prod-deploy?title=myapp-prod"
```

## Troubleshooting

### Artifact Not Found

1. **Check version tags**: Are they compliant with semantic versioning?
2. **Check artifact names**: Do they contain OS/architecture?
3. **Check permissions**: Are authentication credentials correctly configured?

### Debug Methods

```bash
# Enable debug logs to check details
dewy server --registry ghr://owner/repo --log-level debug
```

Registry is a crucial component at the core of Dewy's operation. Select the appropriate registry type for your use case and build an efficient deployment environment.
