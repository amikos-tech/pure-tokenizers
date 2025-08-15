# Deployment Guide

This guide covers the deployment and release process for the CGo-free Tokenizers project.

## Quick Start

### Creating a Release

1. **Prepare the release:**
   ```bash
   ./scripts/prepare-release.sh v1.0.0
   ```

2. **Push to trigger CI:**
   ```bash
   git push origin main && git push origin v1.0.0
   ```

3. **Monitor the build:**
   - Go to GitHub Actions tab
   - Watch the "Build and Release" workflow
   - Release will be created automatically when complete

## Supported Platforms

The CI system builds for the following platforms:

| Platform | Architecture | Target Triple | Library File |
|----------|-------------|---------------|--------------|
| Linux | x86_64 | `x86_64-unknown-linux-gnu` | `libtokenizers.so` |
| Linux | ARM64 | `aarch64-unknown-linux-gnu` | `libtokenizers.so` |
| macOS | Intel | `x86_64-apple-darwin` | `libtokenizers.dylib` |
| macOS | Apple Silicon | `aarch64-apple-darwin` | `libtokenizers.dylib` |
| Windows | x86_64 | `x86_64-pc-windows-msvc` | `libtokenizers.dll` |

## Release Assets

Each release includes:

- **Platform-specific archives**: `libtokenizers-{arch}-{platform}.tar.gz`
- **Checksum files**: `libtokenizers-{arch}-{platform}.tar.gz.sha256`
- **Automatic release notes**: Generated from commits and PRs

## Download Integration

The Go library automatically downloads the appropriate platform library:

```go
// Automatic download
tokenizer, err := tokenizers.FromFile("config.json", 
    tokenizers.WithDownloadLibrary())

// Manual path
tokenizer, err := tokenizers.FromFile("config.json", 
    tokenizers.WithLibraryPath("/path/to/lib"))
```

## Environment Variables

### For Users

- `TOKENIZERS_LIB_PATH`: Override library path
- `TOKENIZERS_GITHUB_REPO`: Custom repository for downloads
- `TOKENIZERS_VERSION`: Specific version to download

### For CI/CD

- `GITHUB_TOKEN`: Automatically provided for releases
- `CARGO_TERM_COLOR`: Enables colored output

## Local Development

### Setup

```bash
# Install development dependencies
make dev-setup

# Check environment
make check-env
```

### Building

```bash
# Quick build and test
./scripts/build-local.sh test

# Build for all targets (requires setup)
make build-all-targets

# Create release assets locally
make create-release-assets
```

### Testing

```bash
# Run all tests
make test-v2

# Test download functionality
make test-download

# Test Rust components
make test-rust
```

## CI/CD Workflows

### 1. Continuous Integration (`ci.yml`)
- **Trigger**: Every push/PR
- **Purpose**: Basic testing and validation
- **Platforms**: Linux, macOS, Windows

### 2. Build and Release (`build-and-release.yml`)
- **Trigger**: Git tags (`v*`)
- **Purpose**: Create releases with all platform assets
- **Features**: Cross-compilation, checksum generation, automatic releases

### 3. Cross Compilation Test (`cross-compile.yml`)
- **Trigger**: Changes to Rust code
- **Purpose**: Verify cross-compilation works
- **Targets**: All supported platforms + additional variants

### 4. Download Test (`test-download.yml`)
- **Trigger**: Weekly schedule, manual dispatch
- **Purpose**: Test download functionality with real releases

## Release Process Details

### Automatic Process (Recommended)

1. **Version Update**: Script updates `Cargo.toml`
2. **Commit and Tag**: Creates commit and git tag
3. **Push**: Triggers GitHub Actions
4. **Build**: Cross-compiles for all platforms
5. **Test**: Validates libraries work correctly
6. **Package**: Creates tar.gz archives with checksums
7. **Release**: Creates GitHub release with all assets

### Manual Process

If you need to create a release manually:

1. Update version in `Cargo.toml`
2. Commit changes
3. Create and push tag: `git tag v1.0.0 && git push origin v1.0.0`
4. GitHub Actions handles the rest

## Troubleshooting

### Common Issues

1. **Build Failures**
   - Check Rust toolchain is up to date
   - Verify all targets are installed
   - Check for dependency conflicts

2. **Test Failures**
   - Ensure library path is set correctly
   - Verify library is compatible with current platform
   - Check for missing dependencies

3. **Release Failures**
   - Verify tag format is correct (`v*`)
   - Check GitHub token permissions
   - Ensure all required files are present

### Debug Commands

```bash
# Check library compatibility
file target/release/libtokenizers.*

# Test library loading
go test -v -run TestLibraryValidation

# Verify download functionality
go test -v -run TestDownloadFunctionality
```

## Security Considerations

- **Checksum Verification**: All downloads are verified
- **HTTPS Only**: All network requests use HTTPS
- **Minimal Permissions**: CI tokens have minimal required permissions
- **Dependency Scanning**: Regular dependency updates

## Monitoring

- **Build Status**: GitHub Actions provides build status
- **Download Stats**: Available in GitHub repository insights
- **Error Tracking**: CI failures are reported via GitHub notifications

## Best Practices

1. **Test Before Release**: Always run full test suite
2. **Semantic Versioning**: Follow semver for version numbers
3. **Release Notes**: Include meaningful release notes
4. **Platform Testing**: Test on multiple platforms when possible
5. **Dependency Updates**: Keep dependencies up to date

## Support

For deployment issues:

1. Check the [CI/CD documentation](CI-CD.md)
2. Review GitHub Actions logs
3. Test locally with provided scripts
4. Open an issue with detailed error information