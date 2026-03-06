# Release Guide

This guide provides step-by-step instructions for creating releases of the pure-tokenizers project.

## Overview

The project uses **separate release cycles** for Rust and Go components:
- **Rust releases** (`rust-vX.Y.Z`): Platform-specific library binaries
- **Go releases** (`vX.Y.Z`): Go module with public API

Both must be released together for feature additions, with Rust released first.

## Prerequisites

- Git access with push permissions
- GitHub CLI (`gh`) installed and authenticated
- Understanding of [semantic versioning](https://semver.org/)
- Clean working directory (`git status` shows no uncommitted changes)

## Release Process

### Step 1: Update Version in Cargo.toml

Update the version number in `Cargo.toml`:

```toml
[package]
name = "tokenizers"
version = "0.1.2"  # Update this
edition = "2021"
```

**Version Selection Guide:**
- **Patch** (0.1.X): Bug fixes, no new features, no breaking changes
- **Minor** (0.X.0): New features, no breaking changes
- **Major** (X.0.0): Breaking changes to public API or FFI

### Step 2: Commit and Push Version Bump

```bash
# Add the change
git add Cargo.toml

# Commit with conventional commit message
git commit -m "chore: bump version to 0.1.2"

# Push to main
git push
```

### Step 3: Create Rust Release

```bash
# Create and push Rust release tag
git tag rust-v0.1.2
git push origin rust-v0.1.2
```

This triggers the `rust-release.yml` workflow which:
- Builds library binaries for all supported platforms
- Creates release with artifacts
- Takes approximately 3-5 minutes

**Monitor the build:**
```bash
# Watch the workflow
gh run list --limit 5

# Get the run ID for "Rust Release" and watch it
gh run watch <RUN_ID>
```

### Step 4: Wait for Rust Release to Complete

**Critical:** Do not proceed until the Rust release workflow completes successfully.

**Verify Rust release:**
```bash
gh release view rust-v0.1.2
```

You should see 7 platform-specific assets:
- `libtokenizers-aarch64-apple-darwin.tar.gz`
- `libtokenizers-aarch64-unknown-linux-gnu.tar.gz`
- `libtokenizers-aarch64-unknown-linux-musl.tar.gz`
- `libtokenizers-x86_64-apple-darwin.tar.gz`
- `libtokenizers-x86_64-pc-windows-msvc.tar.gz`
- `libtokenizers-x86_64-unknown-linux-gnu.tar.gz`
- `libtokenizers-x86_64-unknown-linux-musl.tar.gz`

### Step 5: Create Go Release

```bash
# Create and push Go release tag
git tag v0.1.2
git push origin v0.1.2
```

This triggers the `go-release.yml` workflow which:
- Downloads the Rust v0.1.2 artifacts
- Runs tests on multiple platforms
- Creates the Go module release
- Takes approximately 2-3 minutes

**Monitor the build:**
```bash
gh run watch <GO_RELEASE_RUN_ID>
```

### Step 6: Verify Releases

**Check Go release:**
```bash
gh release view v0.1.2
```

**Test the release:**
```bash
# In a test project
go get github.com/amikos-tech/pure-tokenizers@v0.1.2
```

## Troubleshooting

### Go Release Fails with "undefined symbol" Error

**Symptom:** Go release tests fail with errors like:
```
undefined symbol: encode_batch_pairs
```

**Cause:** The Go release workflow started before the Rust release finished, so it fell back to building the Rust library locally from the wrong commit.

**Solution:** Re-run the failed workflow:
```bash
# Find the failed run ID
gh run list --workflow="Go Release" --limit 5

# Re-run failed jobs
gh run rerun <RUN_ID> --failed
```

The re-run will now download the correct Rust artifacts and should succeed.

### Checking Workflow Status

```bash
# List recent runs
gh run list --limit 10

# View specific run details
gh run view <RUN_ID>

# View failed logs
gh run view <RUN_ID> --log-failed

# Watch a running workflow
gh run watch <RUN_ID>
```

### Release Already Exists

If you need to recreate a release:

```bash
# Delete the release and tag
gh release delete v0.1.2 --yes
git tag -d v0.1.2
git push origin :refs/tags/v0.1.2

# Recreate following the normal process
```

### Wrong Version Tagged

If you tagged the wrong commit:

```bash
# Delete the remote tag
git push origin :refs/tags/v0.1.2

# Delete local tag
git tag -d v0.1.2

# Tag the correct commit
git checkout <correct-commit-sha>
git tag v0.1.2
git push origin v0.1.2
```

## Quick Reference

### Complete Release Commands

```bash
# 1. Update version
vim Cargo.toml  # Change version to X.Y.Z

# 2. Commit and push
git add Cargo.toml
git commit -m "chore: bump version to X.Y.Z"
git push

# 3. Create Rust release
git tag rust-vX.Y.Z
git push origin rust-vX.Y.Z

# 4. Wait and verify Rust release
gh run watch <RUST_RUN_ID>
gh release view rust-vX.Y.Z

# 5. Create Go release
git tag vX.Y.Z
git push origin vX.Y.Z

# 6. Verify Go release
gh run watch <GO_RUN_ID>
gh release view vX.Y.Z
```

### Monitoring Commands

```bash
# List recent workflow runs
gh run list --limit 5

# Watch specific run
gh run watch <RUN_ID>

# View release details
gh release view <TAG>

# List all releases
gh release list
```

## Supported Platforms

Each Rust release includes binaries for:

| Platform | Architecture | File Extension |
|----------|-------------|----------------|
| Linux (GNU) | x86_64 | `.so` |
| Linux (GNU) | aarch64 | `.so` |
| Linux (MUSL) | x86_64 | `.so` |
| Linux (MUSL) | aarch64 | `.so` |
| macOS | x86_64 | `.dylib` |
| macOS | Apple Silicon | `.dylib` |
| Windows | x86_64 | `.dll` |

## Release Checklist

- [ ] Version updated in `Cargo.toml`
- [ ] Version bump committed with conventional commit message
- [ ] Changes pushed to main
- [ ] Rust release tag created and pushed
- [ ] Rust release workflow completed successfully
- [ ] All 7 platform artifacts present in Rust release
- [ ] Go release tag created and pushed
- [ ] Go release workflow completed successfully
- [ ] Both releases verified with `gh release view`
- [ ] Test installation: `go get github.com/amikos-tech/pure-tokenizers@vX.Y.Z`

## Common Mistakes to Avoid

1. **Creating Go release before Rust completes** - Always wait for Rust release to finish
2. **Mismatched versions** - Cargo.toml version must match the tag versions (without the `rust-` prefix)
3. **Skipping version bump commit** - Always commit version changes before tagging
4. **Not verifying artifacts** - Always check that all 7 platform binaries are present
5. **Using wrong branch** - Always release from `main` branch

## Additional Resources

- [CI/CD Documentation](CI-CD.md)
- [Deployment Guide](DEPLOYMENT.md)
- [Semantic Versioning](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
