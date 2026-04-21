# Contributing to Dewy

Thank you for your interest in contributing to Dewy! This document describes how to contribute to the project.

## Code of Conduct

Please be respectful and constructive in all interactions. We aim to maintain a welcoming community for everyone.

## Getting Started

### Prerequisites

- Go (version listed in [`go.mod`](./go.mod))
- `make`
- Docker (for container-related tests)

### Clone and Build

```sh
git clone https://github.com/linyows/dewy.git
cd dewy
make build
```

## Development Workflow

### Running Tests

Run the full test suite:

```sh
make test
```

Run integration tests for the container package:

```sh
make images      # Build test container images
make integration
```

### Linting

```sh
make lint
```

All code must pass `golangci-lint` before being merged.

### Running Locally

The `Makefile` provides targets to run Dewy against a test application:

```sh
make server     # Run in server mode
make assets     # Run in assets mode
make container  # Run in container mode
```

## Reporting Issues

Before filing a new issue, please search existing issues to avoid duplicates. When reporting a bug, include:

- Dewy version (`dewy --version`)
- Operating system and architecture
- Deployment mode (server/assets/container) and registry/artifact store in use
- Minimal steps to reproduce
- Expected vs. actual behavior
- Relevant logs (with `-l debug` if possible)

For security vulnerabilities, please follow the process in [SECURITY.md](./SECURITY.md) instead of opening a public issue.

## Submitting Pull Requests

1. Fork the repository and create a topic branch from `main`.
2. Make your changes, following the guidelines below.
3. Add or update tests that cover your changes.
4. Ensure `make test` and `make lint` both pass.
5. Open a pull request against `main`.

### Pull Request Guidelines

- **Write PR titles and descriptions in English.**
- Keep PRs focused: one logical change per PR.
- Describe the motivation and summarize the change in the PR body.
- Reference related issues (e.g., `Closes #123`).
- Update documentation and README examples when user-facing behavior changes.
- Ensure CI passes before requesting review.

### Commit Messages

- Use clear, imperative subject lines (e.g., "Add support for X", "Fix Y when Z").
- Keep the subject concise; use the body to explain *why* when it isn't obvious from the diff.

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Follow the existing patterns and conventions in the codebase.
- Use meaningful variable and function names.
- All files must end with a trailing newline (LF).

### Tests

- Add unit tests for new functionality.
- Include integration tests when changing behavior that spans packages (e.g., container deployment flows).
- Cover edge cases and error paths.

## Adding New Components

Dewy's architecture is composed of pluggable components: registries, artifact stores, cache stores (KVS), and notifiers. When adding a new implementation:

- Place it under the appropriate package (`registry/`, `artifact/`, `kvs/`, `notifier/`).
- Implement the relevant interface defined in that package.
- Register the implementation so it can be selected via the URL scheme (e.g., `ghr://`, `s3://`).
- Add tests and update the README to document the new scheme and options.

## Releasing

Releases are managed by the maintainers via `goreleaser`. Contributors do not need to create release PRs.

## License

By contributing to Dewy, you agree that your contributions will be licensed under the [MIT License](./LICENSE).
