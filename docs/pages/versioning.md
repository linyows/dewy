---
title: Versioning
description: |
  Dewy automatically detects the latest version of applications based on semantic versioning
  and achieves continuous deployment. It provides comprehensive version management functionality including pre-release version management.
---

# {% $markdoc.frontmatter.title %} {% #overview %}

{% $markdoc.frontmatter.description %}

## Overview {% #overview-details %}

Versioning in Dewy is the core functionality of pull-based deployment. Based on version information retrieved from registries, it automatically determines whether new versions are available by comparing with currently running versions, and executes automatic deployment as needed.

**Key Features:**
- Complete support for Semantic Versioning (SemVer)
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

## Dewy's Version Detection Algorithm {% #version-detection %}

### Comparison Rules {% #comparison-rules %}

Dewy implements its own semantic version comparison algorithm:

1. **MAJOR version comparison** - Compare numerically, prioritize larger
2. **MINOR version comparison** - When MAJOR is same, compare numerically
3. **PATCH version comparison** - When MAJOR.MINOR is same, compare numerically
4. **Pre-release version handling**:
   - Official version > Pre-release version
   - Pre-release versions are compared as strings

### Latest Version Determination {% #latest-version %}

For all version tags retrieved from registry:

```go
// Pseudo code
func findLatest(versions []string, allowPreRelease bool) string {
    validVersions := filterValidSemVer(versions, allowPreRelease)
    return findMaxVersion(validVersions)
}
```

**Processing flow:**
1. Semantic version format validation
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
<path-prefix>/<semver>/<artifact>
```

**Configuration example:**
```bash
dewy server --registry "s3://ap-northeast-1/releases/myapp?pre-release=true"
```

**S3 arrangement example:**
```
releases/myapp/v1.2.4/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.4/myapp_darwin_arm64.tar.gz
releases/myapp/v1.2.3/myapp_linux_amd64.tar.gz
releases/myapp/v1.2.3-rc.1/myapp_linux_amd64.tar.gz
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
# ❌ Non-compliant with semantic versioning
git tag release-2024-03-15
git tag latest
git tag stable

# ❌ Irregular naming
git tag v1.2.3-SNAPSHOT
git tag 1.2.3-final
```

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