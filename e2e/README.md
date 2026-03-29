# E2E Tests

End-to-end tests that verify dewy's deployment lifecycle works correctly against real registries and external services. These tests are executed by [Probe](https://github.com/linyows/probe) via GitHub Actions.

## What These Tests Guarantee

The E2E tests ensure that dewy can:

1. **Detect and deploy an initial version** from a registry, creating the correct symlink structure
2. **Detect a new version** published to the registry and perform a rolling update
3. **Notify** deployment events to Slack (server command with Slack notifier)
4. **Manage containers** including health checks, replica scaling, and non-root execution
5. **Handle multiple port mappings** with TCP proxy for container deployments

All of the above are verified across every supported combination of commands and registries:

| Command | Registry | Description |
|---|---|---|
| `server` | GitHub Releases (`ghr`) | Process management with symlink-based deployment |
| `server` | AWS S3 (`s3`) | Same as above, sourced from S3 |
| `server` | Google Cloud Storage (`gs`) | Same as above, sourced from GCS |
| `assets` | GitHub Releases (`ghr`) | Static asset download and symlink management |
| `assets` | AWS S3 (`s3`) | Same as above, sourced from S3 |
| `assets` | Google Cloud Storage (`gs`) | Same as above, sourced from GCS |
| `container` | OCI / ghcr.io (`img`) | Container deployment with 3 replicas |
| `container` | Docker Hub (`img`) | Same as above, sourced from Docker Hub |
| `container` (multi-port) | OCI / ghcr.io (`img`) | Container deployment with 2 replicas and 2 port mappings |

### Verification Details

**Server / Assets commands:**
- No errors in log output
- Two symlinks created (initial + updated version)
- Two versions started/downloaded (initial + new)
- New version string appears in log

**Container command:**
- No errors in log output (excluding transient `context deadline exceeded`)
- Correct number of containers created for both initial and new versions
- Containers run as non-root user

**Container multi-port command:**
- All of the above container checks
- TCP proxy started on each mapped port
- Backends registered for each port with correct replica count
- HTTP health check returns 200 on both ports

## Architecture

```
GitHub Actions (.github/workflows/end-to-end-test.yml)
│
└─ Probe (test runner)
   │
   └─ e2e/test.yml (main test definition)
      │
      │  Phase 0: Setup
      │  ├── Check credentials (GITHUB_TOKEN, AWS, GCP, Slack, Docker Hub)
      │  ├── Generate unique version string (v3.0.0-<unixtime>)
      │  └── Build dewy binaries for all command/registry combinations
      │
      │  Phase 1: Start all dewy instances (parallel)
      │  ├── server  x3 (ghr, s3, gs)  ─── wait for 'Create symlink' in log
      │  ├── assets  x3 (ghr, s3, gs)  ─── wait for 'Create symlink' in log
      │  ├── container x2 (img, dockerhub) ── wait for 'Starting new container'
      │  └── container-multiport x1 (img) ── wait for 'Starting new container'
      │
      │  Phase 2: Trigger update
      │  └── Create a new GitHub Release on linyows/dewy-testapp
      │
      │  Phase 3: Verify all instances (parallel)
      │  ├── server-verify.yml  x3 ─── new version detected, symlink updated
      │  ├── assets-verify.yml  x3 ─── new version downloaded, symlink updated
      │  ├── container-verify.yml x2 ── new containers created, non-root check
      │  └── container-multiport-verify.yml x1 ── ports, proxies, health checks
      │
      └─ Done
```

### Phase Details

**Phase 0 (Setup)** validates that all required credentials are available, generates a unique version string using the current Unix timestamp, and builds 9 dewy binaries -- one per command/registry combination.

**Phase 1 (Initial Deployment)** starts all 9 dewy instances in parallel as background processes. Each instance polls its registry, discovers the latest pre-release version of `linyows/dewy-testapp`, and performs an initial deployment. The test waits for log evidence that deployment completed before proceeding.

**Phase 2 (Trigger Update)** creates a new GitHub Release on the `linyows/dewy-testapp` repository using the version generated in Phase 0. This simulates a real version publish that all running dewy instances should detect.

**Phase 3 (Verification)** runs all verification jobs in parallel. Each verify job waits for the new version to appear in the instance's log, then stops the process and inspects the log for expected behavior. Verification logic is defined in `jobs/*-verify.yml`.

## Directory Structure

```
e2e/
├── README.md               # This file
├── test.yml                # Main test definition (Probe format)
├── jobs/                   # Reusable verification job definitions
│   ├── server.yml          # Server start + verify (standalone)
│   ├── server-verify.yml   # Server verification steps
│   ├── assets.yml          # Assets start + verify (standalone)
│   ├── assets-verify.yml   # Assets verification steps
│   ├── container.yml       # Container start + verify (standalone)
│   ├── container-verify.yml           # Container verification steps
│   ├── container-multiport.yml        # Multi-port start + verify (standalone)
│   └── container-multiport-verify.yml # Multi-port verification steps
├── server/{ghr,s3,gs}/     # Working directories for server tests
├── assets/{ghr,s3,gs}/     # Working directories for assets tests
├── container/{img,dockerhub}/         # Working directories for container tests
└── container-multiport/img/           # Working directory for multi-port test
```

## Running

The tests are triggered by GitHub Actions in two ways:

- **Issue comment**: posting `/e2e` on an issue or pull request
- **Workflow dispatch**: manually from the Actions tab

To visualize the test DAG:

```bash
probe dag --mermaid e2e/test.yml
```
