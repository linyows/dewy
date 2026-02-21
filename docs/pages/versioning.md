---
title: Versioning
description: |
  Dewy automatically detects the latest version of applications based on semantic versioning or calendar versioning
  and achieves continuous deployment. It provides comprehensive version management functionality including pre-release version management.
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## Overview {% #overview-details %}

Versioning in Dewy is the core functionality of pull-based deployment. Based on version information retrieved from registries, it automatically determines whether new versions are available by comparing with currently running versions, and executes automatic deployment as needed.

**Key Features:**
- Complete support for Semantic Versioning (SemVer)
- Calendar Versioning (CalVer) support with flexible format specifiers
- Flexible management of pre-release versions
- Support for multiple version formats (`v1.2.3` / `1.2.3`)
- Support for environment-specific version strategies

## Semantic Versioning Basics {% #semantic-versioning %}

### Version Format {% #version-format %}

Dewy supports version management compliant with [Semantic Versioning 2.0.0](https://semver.org/).

**Basic format:**
```
MAJOR.MINOR.PATCH
```

**Examples:**
- `1.2.3` - Version 1.2.3
- `v1.2.3` - Version 1.2.3 with v prefix
- `2.0.0` - Major version 2.0.0

### Version Number Meanings

{% table %}
* Type
* Description
* Increment Condition
---
* MAJOR
* Incompatible changes
* Breaking API changes, architecture overhaul
---
* MINOR
* Backward-compatible feature additions
* New feature additions, existing feature extensions
---
* PATCH
* Backward-compatible bug fixes
* Bug fixes, security fixes
{% /table %}

### Pre-release Versions {% #pre-release %}

Pre-release versions are versions intended for testing and evaluation before official release.

**Format:**
```
MAJOR.MINOR.PATCH-<pre-release-identifier>
```

**Common patterns:**
- `v1.2.3-alpha` - Alpha version (initial testing)
- `v1.2.3-beta.1` - Beta version 1 (feature-complete testing)
- `v1.2.3-rc.1` - Release candidate 1 (final verification)

{% callout type="note" title="Pre-release Version Priority" %}
Pre-release versions are treated with lower priority than official versions of the same MAJOR.MINOR.PATCH.
Example: `v1.2.3-rc.1 < v1.2.3`
{% /callout %}

### Build Metadata and Deployment Slots {% #build-metadata %}

Semantic versioning also supports build metadata, which is appended with a `+` sign. Dewy uses build metadata for **deployment slot** management, enabling blue/green deployment patterns.

**Format:**
```
MAJOR.MINOR.PATCH+<build-metadata>
MAJOR.MINOR.PATCH-<pre-release>+<build-metadata>
```

**Common patterns:**
- `v1.2.3+blue` - Stable version for Blue slot
- `v1.2.3+green` - Stable version for Green slot
- `v1.2.3-rc.1+blue` - Pre-release for Blue slot

{% callout type="note" title="Build Metadata and Version Comparison" %}
According to semantic versioning specification, build metadata is **ignored** during version comparison.
This means `v1.2.3+blue` and `v1.2.3+green` are considered the **same version**.
Dewy uses the `--slot` option to filter which versions to deploy based on build metadata.
{% /callout %}

**Usage:**
```bash
# Blue environment - only deploys versions with +blue metadata
dewy server --registry ghr://owner/repo --slot blue -- /opt/myapp/current/myapp

# Green environment - only deploys versions with +green metadata
dewy server --registry ghr://owner/repo --slot green -- /opt/myapp/current/myapp

# Without --slot - deploys any version (backward compatible)
dewy server --registry ghr://owner/repo -- /opt/myapp/current/myapp
```

## Calendar Versioning (CalVer) {% #calver %}

In addition to SemVer, Dewy supports [Calendar Versioning (CalVer)](https://calver.org/)—a versioning scheme based on release dates rather than compatibility semantics.

### CalVer Format {% #calver-format %}

CalVer is enabled by specifying a format string via the `--calver` option. The format consists of specifiers separated by dots.

**Supported specifiers:**

{% table %}
* Specifier
* Description
* Example
---
* YYYY
* Full year
* 2024
---
* YY
* Short year (no padding)
* 6, 16, 106
---
* 0Y
* Zero-padded short year
* 06, 16, 106
---
* MM
* Month (no padding)
* 1, 11
---
* 0M
* Zero-padded month
* 01, 11
---
* WW
* Week (no padding)
* 1, 33, 52
---
* 0W
* Zero-padded week
* 01, 33, 52
---
* DD
* Day (no padding)
* 1, 9, 31
---
* 0D
* Zero-padded day
* 01, 09, 31
---
* MICRO
* Incremental number
* 0, 1, 42
{% /table %}

**Format examples:**
- `YYYY.0M.0D.MICRO` - Year, zero-padded month, zero-padded day, micro (e.g., `2024.01.15.3`)
- `YYYY.MM.DD` - Year, month, day (e.g., `2024.1.9`)
- `YYYY.0M.MICRO` - Year, zero-padded month, micro (e.g., `2024.06.3`)

### CalVer Usage {% #calver-usage %}

```bash
# CalVer with GitHub Releases
dewy server --registry ghr://owner/repo --calver YYYY.0M.0D.MICRO -- /opt/myapp/current/myapp

# CalVer with S3
dewy server --registry "s3://ap-northeast-1/releases/myapp" --calver YYYY.0M.MICRO -- /opt/myapp/current/myapp

# CalVer with pre-release versions
dewy server --registry "ghr://owner/repo?pre-release=true" --calver YYYY.0M.0D.MICRO -- /opt/myapp/current/myapp
```

### Pre-release and Build Metadata with CalVer {% #calver-metadata %}

CalVer supports pre-release identifiers and build metadata just like SemVer:

```
<calver>-<pre-release>+<build-metadata>
```

**Examples:**
- `2024.01.15.3-rc.1` - Release candidate
- `2024.06.0+blue` - Blue deployment slot
- `v2024.01.15.3-beta.2+green` - Pre-release for Green slot with v prefix

{% callout type="note" title="CalVer and Blue/Green Deployment" %}
Build metadata (`+blue`, `+green`) and pre-release identifiers work identically for both SemVer and CalVer. All deployment patterns (blue/green, staging, canary) are fully supported with CalVer.
{% /callout %}

## Dewy's Version Detection Algorithm {% #version-detection %}

### Comparison Rules {% #comparison-rules %}

Dewy implements version comparison algorithms for both SemVer and CalVer:

**SemVer comparison:**

1. **MAJOR version comparison** - Compare numerically, prioritize larger
2. **MINOR version comparison** - When MAJOR is same, compare numerically
3. **PATCH version comparison** - When MAJOR.MINOR is same, compare numerically
4. **Pre-release version handling** - Official version > Pre-release version; pre-release versions are compared as strings

**CalVer comparison:**

1. **Segment-by-segment comparison** - Each segment is compared numerically from left to right
2. **Pre-release version handling** - Same as SemVer: official version > pre-release version

### Latest Version Determination {% #latest-version %}

For all version tags retrieved from registry:

```go
// Pseudo code
func findLatest(versions []string, allowPreRelease bool, calverFormat string) string {
    if calverFormat != "" {
        validVersions := filterValidCalVer(versions, calverFormat, allowPreRelease)
        return findMaxVersion(validVersions)
    }
    validVersions := filterValidSemVer(versions, allowPreRelease)
    return findMaxVersion(validVersions)
}
```

**Processing flow:**
1. Version format validation (SemVer or CalVer based on `--calver` option)
2. Filtering by pre-release settings
3. Numerical comparison and sorting
4. Maximum value selection

## Registry-specific Version Management {% #registry-versioning %}

### GitHub Releases {% #github-releases %}

Automatically detects versions from GitHub release tag names.

```bash
# Stable versions only (default)
dewy server --registry ghr://owner/repo

# Including pre-release versions
dewy server --registry "ghr://owner/repo?pre-release=true"

# Using CalVer format
dewy server --registry ghr://owner/repo --calver YYYY.0M.0D.MICRO
```

**Grace period consideration:**

{% callout type="important" title="CI/CD Support" %}
After release creation with GitHub Actions, artifact building and placement may take time.
Dewy sets a 30-minute grace period for new releases and does not notify "artifact not found" errors during this time.
{% /callout %}

### AWS S3 {% #aws-s3 %}

Extracts versions from S3 object path structure.

**Required path structure:**
```
<path-prefix>/<version>/<artifact>
```

**Configuration example:**
```bash
# SemVer (default)
dewy server --registry "s3://ap-northeast-1/releases/myapp?pre-release=true"

# CalVer
dewy server --registry "s3://ap-northeast-1/releases/myapp" --calver YYYY.0M.MICRO
```

**S3 arrangement example (SemVer):**
```
releases/myapp/v1.2.4/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.4/myapp_darwin_arm64.tar.gz
releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.3-rc.1/myapp_linux_amd64.tar.gz
```

**S3 arrangement example (CalVer):**
```
releases/myapp/2024.06.15.0/myapp_linux_amd64.tar.gz
releases/myapp/2024.06.15.1/myapp_linux_amd64.tar.gz
releases/myapp/2024.07.01.0/myapp_linux_amd64.tar.gz
```

## Environment-specific Version Strategies {% #environment-strategies %}

### Production Environment {% #production %}

**Recommended settings:**
```bash
# Auto-deploy stable versions only
dewy server --registry ghr://company/myapp \
  --interval 300s \
  --log-format json -- /opt/myapp/current/myapp
```

**Features:**
- Exclude pre-release versions (`pre-release=false`)
- Longer polling intervals to reduce system load
- Prioritize monitoring ease with structured logs

### Staging Environment {% #staging %}

**Recommended settings:**
```bash
# Include pre-release versions for early testing
dewy server --registry "ghr://company/myapp?pre-release=true" \
  --interval 60s \
  --notifier "slack://staging-deploy?title=MyApp+Staging" \
  -- /opt/myapp/current/myapp
```

**Features:**
- Actively incorporate pre-release versions
- Short polling intervals for quick feedback
- Share with entire team through deployment notifications

### Blue/Green Deployment {% #blue-green %}

For blue/green deployment patterns, use build metadata to manage deployment slots:

**Recommended settings:**
```bash
# Blue environment
dewy server --registry ghr://company/myapp --slot blue \
  --interval 60s \
  --notifier "slack://production-deploy?title=MyApp+Blue" \
  -- /opt/myapp/current/myapp

# Green environment
dewy server --registry ghr://company/myapp --slot green \
  --interval 60s \
  --notifier "slack://production-deploy?title=MyApp+Green" \
  -- /opt/myapp/current/myapp
```

**Features:**
- Independent version control for each environment
- Zero-downtime deployment through traffic switching
- Easy rollback by switching traffic back
- Combine with pre-release for canary deployments

**Deployment workflow:**
```bash
# Step 1: Deploy to Green (standby)
gh release create v1.2.0+green --title "v1.2.0 for Green"

# Step 2: Verify Green environment

# Step 3: Switch traffic to Green (via load balancer)

# Step 4: Deploy same version to Blue
gh release create v1.2.0+blue --title "v1.2.0 for Blue"

# Both environments now running v1.2.0
```

## Version Management Best Practices {% #best-practices %}

### Tagging Rules {% #tagging-rules %}

**Recommended tag naming conventions:**

```bash
# Official releases
git tag v1.2.3
git tag v2.0.0

# Pre-releases
git tag v1.3.0-alpha
git tag v1.3.0-beta.1
git tag v1.3.0-rc.1

# Security fixes
git tag v1.2.4  # Security fix version for 1.2.3
```

**Patterns to avoid:**
```bash
# ❌ Non-structured naming
git tag latest
git tag stable

# ❌ Irregular naming
git tag v1.2.3-SNAPSHOT
git tag 1.2.3-final
```

{% callout type="note" title="Date-based Tags" %}
Date-based tags like `2024.03.15.0` are now supported with the `--calver` option. See [Calendar Versioning](#calver) for details.
{% /callout %}

### Release Strategy {% #release-strategy %}

**Staged release pattern:**

1. **alpha** - Testing by internal developers
2. **beta** - Testing by limited users
3. **rc** (Release Candidate) - Testing in production-like conditions
4. **Official version** - Production environment deployment

**Example:**
```bash
v2.1.0-alpha    → Development environment
v2.1.0-beta.1   → Staging environment
v2.1.0-rc.1     → Staging environment (production-equivalent configuration)
v2.1.0          → Production environment
```

## Troubleshooting {% #troubleshooting %}

### Common Issues and Solutions {% #common-issues %}

**Version not detected:**

```bash
# Debug: Check available tags
curl -s https://api.github.com/repos/owner/repo/releases \
  | jq -r '.[].tag_name'

# Check detection process in logs
dewy server --log-format json -l debug --registry ghr://owner/repo
```

**Unexpected version selected:**

```bash
# Check pre-release settings
dewy server --registry "ghr://owner/repo?pre-release=false"  # Stable only
dewy server --registry "ghr://owner/repo?pre-release=true"   # Include pre-release
```

**Access permission issues:**

```bash
# Check GitHub Token
echo $GITHUB_TOKEN | cut -c1-10  # Display only first 10 characters
gh auth status  # Check authentication status with GitHub CLI
```

## Related Topics {% #related %}

- [Registry](/registry) - Version detection source configuration and registry details
- [Cache](/cache) - Version information and artifact storage management
- [Architecture](/architecture) - Dewy's overall configuration and deployment process
- [FAQ](/faq) - Frequently asked questions about versioning