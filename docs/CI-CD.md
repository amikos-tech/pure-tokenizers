# CI/CD Documentation

This document describes the continuous integration and deployment setup for the CGo-free Tokenizers project.

## Overview

The project uses GitHub Actions for CI/CD with multiple workflows to ensure code quality, cross-platform compatibility, and automated releases.

## Workflows

### 1. CI Workflow (`.github/workflows/ci.yml`)

**Triggers:** Push to main/master, Pull Requests

**Purpose:** Basic continuous integration testing

**Jobs:**
- **rust-test**: Runs Rust tests, linting, and formatting checks
- **go-test**: Tests Go bindings on multiple platforms (Ubuntu, macOS, Windows)
- **integration-test**: Tests the integration between Rust and Go components

**Key Features:**
- Cross-platform testing (Linux, macOS, Windows)
- Rust formatting and clippy linting
- Go linting with golangci-lint
- Caching for faster builds

### 2. Build and Release Workflow (`.github/workflows/build-and-release.yml`)

**Triggers:** Git tags starting with 'v', Pull Requests, Manual dispatch

**Purpose:** Build libraries for all supported platforms and create releases

**Supported Platforms:**
- Linux: `x86_64-unknown-linux-gnu`, `aarch64-unknown-linux-gnu`
- macOS: `x86_64-apple-darwin`, `aarch64-apple-darwin`
- Windows: `x86_64-pc-windows-msvc`

**Jobs:**
- **build**: Cross-compiles for all target platforms
- **test**: Tests Go bindings with built libraries
- **release**: Creates GitHub releases with all platform assets

**Artifacts:**
- Platform-specific tar.gz archives containing the shared libraries
- SHA256 checksum files for each archive
- Automatic GitHub release creation for tagged versions

### 3. Cross Compilation Test (`.github/workflows/cross-compile.yml`)

**Triggers:** Changes to Rust source code or build configuration

**Purpose:** Verify cross-compilation works for all targets

**Features:**
- Tests compilation for all supported targets
- Includes additional targets like musl variants
- Verifies library files are created correctly

### 4. Download Functionality Test (`.github/workflows/test-download.yml`)

**Triggers:** Weekly schedule, Manual dispatch

**Purpose:** Test the automatic download functionality with real releases

**Features:**
- Tests download functionality on all platforms
- Can test specific versions via manual dispatch
- Validates the complete download-verify-extract cycle

## Release Process

### Automated Release (Recommended)

1. **Prepare Release:**
   ```bash
   ./scripts/prepare-release.sh v1.0.0
   ```

2. **Push Changes:**
   ```bash
   git push origin main && git push origin v1.0.0
   ```

3. **GitHub Actions automatically:**
   - Builds libraries for all platforms
   - Runs comprehensive tests
   - Creates release with all assets
   - Generates release notes

### Manual Release

1. **Update version in `Cargo.toml`**
2. **Commit and tag:**
   ```bash
   git add Cargo.toml
   git commit -m "chore: bump version to v1.0.0"
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin main && git push origin v1.0.0
   ```

## Local Development

### Setup Development Environment

```bash
make dev-setup
```

This installs:
- Rust targets for cross-compilation
- Required Rust components (rustfmt, clippy)
- Go testing tools
- Linting tools

### Build and Test Locally

```bash
# Quick build and test
./scripts/build-local.sh test

# Or step by step
make build
make test-v2

# Test specific functionality
make test-download
```

### Check Environment

```bash
make check-env
```

## Asset Naming Convention

The CI system creates assets with the following naming pattern:

```
libtokenizers-{arch}-{platform}.tar.gz
libtokenizers-{arch}-{platform}.tar.gz.sha256
```

Examples:
- `libtokenizers-x86_64-unknown-linux-gnu.tar.gz`
- `libtokenizers-aarch64-apple-darwin.tar.gz`
- `libtokenizers-x86_64-pc-windows-msvc.tar.gz`

## Environment Variables

### CI Environment Variables

- `CARGO_TERM_COLOR=always`: Enables colored Cargo output
- `TOKENIZERS_LIB_PATH`: Path to the shared library for testing
- `GITHUB_TOKEN`: Automatically provided for release creation

### User Environment Variables

- `TOKENIZERS_GITHUB_REPO`: Override GitHub repository for downloads
- `TOKENIZERS_VERSION`: Specify version to download
- `TOKENIZERS_LIB_PATH`: Override library path

## Caching Strategy

The workflows use GitHub Actions caching to speed up builds:

- **Cargo registry and git cache**: Shared across jobs
- **Target directory**: Platform and target-specific
- **Go module cache**: Shared across Go jobs

## Security Considerations

- **Checksum Verification**: All releases include SHA256 checksums
- **HTTPS Downloads**: All network requests use HTTPS
- **Token Permissions**: GitHub tokens have minimal required permissions
- **Dependency Pinning**: Actions are pinned to specific versions

## Troubleshooting

### Build Failures

1. **Cross-compilation issues**: Check target installation
2. **Test failures**: Verify library path is set correctly
3. **Release failures**: Ensure tag format is correct (`v*`)

### Common Issues

- **Library not found**: Check `TOKENIZERS_LIB_PATH` environment variable
- **Permission denied**: Ensure scripts have execute permissions
- **Network timeouts**: Download tests may fail due to network issues

### Debug Commands

```bash
# Check library info
go test -v -run TestGetLibraryInfo

# Test download functionality
go test -v -run TestDownloadFunctionality

# Verify cross-compilation
make build-all-targets
```

## Contributing

When contributing:

1. Ensure all CI checks pass
2. Test on multiple platforms when possible
3. Update documentation for new features
4. Follow semantic versioning for releases

## Monitoring

- **CI Status**: Check GitHub Actions tab
- **Release Status**: Monitor releases page
- **Download Stats**: Available in GitHub insights
- **Test Coverage**: Generated in CI artifacts