# Contributing to Discovery Agent

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing.

Note that this repository holds two separate components:

- disco-agent: For CyberArk DisCo
- venafi-kubernetes-agent: For TLSPK / Certificate Manager SaaS

## Table of Contents

- [Getting Started](#getting-started)
- [Development Environment](#development-environment)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting a Pull Request](#submitting-a-pull-request)
- [Code Review Process](#code-review-process)
- [Additional Resources](#additional-resources)

### Prerequisites

Before you begin, ensure you have the following installed:

- [Go](https://golang.org/doc/install) (version specified in `go.mod`)
- [Make](https://www.gnu.org/software/make/)
- [Git](https://git-scm.com/)
- [Docker](https://docs.docker.com/get-docker/) (for building container images)

To check which Go version will be used:

```bash
make which-go
```

It's also possible to use a vendored version of Go, via `make vendor-go`.

### Repository Tooling

Most of the setup logic for provisioning tooling and for handling builds and testing
is defined in Makefile logic.

Specifically, `the make/_shared` directory contains shared Makefile logic derived from
the cert-manager [makefile-modules](https://github.com/cert-manager/makefile-modules/) project.

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub

2. **Clone your fork:**

   ```bash
   git clone git@github.com:YOUR-USERNAME/jetstack-secure.git
   cd jetstack-secure
   ```

3. **Add the upstream remote:**

   ```bash
   git remote add upstream git@github.com:jetstack/jetstack-secure.git
   ```

4. **Run initial verification:**

   ```bash
   make verify
   ```

   This ensures your environment is set up correctly.

## Development Environment

### Local Execution

To build and run the agent locally:

```bash
go run main.go agent --agent-config-file ./path/to/agent/config/file.yaml -p 0h1m0s
```

Example configuration files are available:
- [agent.yaml](./agent.yaml)
- [examples/one-shot-secret.yaml](./examples/one-shot-secret.yaml)
- [examples/cert-manager-agent.yaml](./examples/cert-manager-agent.yaml)

You can also run a local echo server to monitor agent requests:

```bash
go run main.go echo
```

### Useful Make Targets

- `make help` - Show all available make targets
- `make verify` - Run all verification checks (linting, formatting, etc.)
- `make test-unit` - Run unit tests
- `make test-helm` - Run Helm chart tests
- `make generate` - Generate code, documentation, and other artifacts
- `make oci-build-preflight` - Build container image
- `make clean` - Clean all temporary files

## Making Changes

### Creating a Branch

Always create a new branch for your changes:

```bash
git checkout -b feature/your-feature-name
```

Use descriptive branch names:
- `feature/` for new features
- `fix/` for bug fixes
- `docs/` for documentation changes
- `refactor/` for refactoring

### Code Style

This project follows standard Go conventions:

- Run `make verify-golangci-lint` to check your code
- Run `make fix-golangci-lint` to automatically fix some issues
- Ensure all code is formatted with `gofmt`
- Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines
- Most of the conventions are enforced by linters, and violations will prevent code being merged

### Committing Changes

1. **Stage your changes:**

   ```bash
   git add .
   ```

2. **Run verification before committing:**

   ```bash
   make verify
   ```

3. **Commit with a descriptive message:**

   ```bash
   git commit -m "Brief description of your changes"
   ```

   Write clear commit messages:
   - Use the imperative mood ("Add feature" not "Added feature")
   - Keep the first line under 72 characters
   - Add additional context in the body if needed

## Testing

### Running Tests Locally

Before submitting a PR, ensure all tests pass:

```bash
# Run unit tests
make test-unit

# Run Helm tests
make test-helm

# Run all verification checks
make verify
```

### End-to-End Tests

E2E tests run automatically in CI when you add specific labels to your PR:

- Add the `test-e2e` label to trigger GKE-based E2E tests
- Add the `keep-e2e-cluster` label if you need to keep the cluster for debugging (remember to delete it manually afterward to avoid costs)

The E2E test script is located at [hack/e2e/test.sh](./hack/e2e/test.sh).

### Writing Tests

- Add unit tests for all new functionality
- Place tests in `*_test.go` files alongside the code they test
- Use the [testify](https://github.com/stretchr/testify) library for assertions
- Aim for meaningful test coverage, not just high percentages

## Submitting a Pull Request

1. **Push your branch to your fork:**

   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create a Pull Request** on GitHub from your fork to the `master` branch of `jetstack/jetstack-secure`

3. **Fill out the PR description** with:
   - Clear description of the changes
   - Related issue numbers (if applicable)
   - Testing instructions
   - Any breaking changes or special considerations

4. **Ensure CI passes:**
   - All tests must pass
   - Code must pass verification / linting checks
   - No merge conflicts

## Code Review Process

### For All Contributors

- PRs require approval before merging
- Keep PRs focused and reasonably sized
- Update your branch if `master` has moved forward:

  ```bash
  git fetch upstream
  git rebase upstream/master
  git push --force-with-lease origin feature/your-feature-name
  ```

### For CyberArk Contributors

**Contributors from inside CyberArk should reach out to the cert-manager team for reviews for PRs which are passing CI.**

The cert-manager team maintains this project and will provide code reviews and guidance for merging changes.

## Additional Resources

- [Project Documentation](https://docs.cyberark.com/mis-saas/vaas/k8s-components/c-tlspk-agent-overview/)
- [Issue Tracker](https://github.com/jetstack/jetstack-secure/issues)
- [Release Process](./RELEASE.md)
- [cert-manager Community](https://cert-manager.io/docs/contributing/)

## Getting Help

If you need help or have questions:

1. Check existing [issues](https://github.com/jetstack/jetstack-secure/issues) and [documentation](https://docs.cyberark.com/mis-saas/vaas/k8s-components/c-tlspk-agent-overview/)
2. Open a new issue with the `question` label
3. For CyberArk contributors, reach out to the cert-manager team

## License

By contributing, you agree that your contributions will be licensed under the license in the LICENSE file in the root directory of this repository.
