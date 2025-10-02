# Composite Actions

This directory contains reusable composite actions for the pure-tokenizers CI workflows. These actions eliminate redundancy and make the workflows more maintainable.

## Architecture

The composite actions follow a layered approach:

```
┌─────────────────────────────────────┐
│    Workflows (ci.yml, etc.)         │
├─────────────────────────────────────┤
│  High-level composite actions:      │
│  - get-rust-library                 │
├─────────────────────────────────────┤
│  Mid-level composite actions:       │
│  - build-rust-library               │
├─────────────────────────────────────┤
│  Low-level composite actions:       │
│  - setup-rust                       │
│  - setup-cross-compilation          │
└─────────────────────────────────────┘
```

## Available Actions

### setup-rust
**Purpose**: Install and configure Rust toolchain with caching

**Inputs**:
- `components`: Optional Rust components (e.g., `rustfmt, clippy`)
- `targets`: Optional Rust targets for cross-compilation
- `cache-key-suffix`: Optional suffix for cache key customization

**Usage**:
```yaml
- uses: ./.github/actions/setup-rust
  with:
    components: rustfmt, clippy
    targets: x86_64-unknown-linux-gnu
```

**Features**:
- Automatic cargo cache management
- Customizable cache keys for better cache isolation

---

### setup-cross-compilation
**Purpose**: Install cross-compilation tools and dependencies

**Features**:
- Installs `cross` from crates.io (stable release)
- Installs platform-specific dependencies (Linux only)
- Caches the `cross` binary for faster subsequent runs

**Usage**:
```yaml
- uses: ./.github/actions/setup-cross-compilation
```

**Note**: Only runs on Linux; automatically skipped on other platforms.

---

### build-rust-library
**Purpose**: Build Rust library for a specified target platform

**Inputs**:
- `target` (required): Rust target triple (e.g., `x86_64-unknown-linux-gnu`)
- `use-cross`: Use cross for building
  - `'auto'` (default): Auto-detect - uses cross on Linux, native elsewhere
  - `'true'`: Force use of cross
  - `'false'`: Force native cargo
- `use-zigbuild`: Use cargo-zigbuild instead (`'true'` or `'false'`)

**Outputs**:
- `library-path`: Path to the built library file

**Usage Examples**:
```yaml
# Linux with cross (auto-detected)
- uses: ./.github/actions/build-rust-library
  with:
    target: x86_64-unknown-linux-gnu

# Force native cargo on Linux
- uses: ./.github/actions/build-rust-library
  with:
    target: x86_64-unknown-linux-gnu
    use-cross: 'false'

# Use zigbuild
- uses: ./.github/actions/build-rust-library
  with:
    target: x86_64-unknown-linux-gnu
    use-zigbuild: 'true'
```

**Build Selection Logic**:
1. If `use-zigbuild='true'` → uses cargo-zigbuild
2. Else if Linux + `use-cross='true'|'auto'` → uses cross
3. Else → uses native cargo

---

### get-rust-library
**Purpose**: Download latest Rust library release or build locally as fallback

**Inputs**:
- `github-token` (required): GitHub token for downloading releases
- `repository` (required): GitHub repository (owner/repo format)

**Features**:
- Downloads from latest `rust-v*` release
- **Security**: Verifies SHA256 checksums for supply chain security
- Automatic fallback to local build if:
  - No releases found
  - Download fails
  - Checksum verification fails
- Platform-aware (Linux, macOS, Windows)
- Sets `TOKENIZERS_LIB_PATH` environment variable

**Usage**:
```yaml
- uses: ./.github/actions/get-rust-library
  with:
    github-token: ${{ github.token }}
    repository: ${{ github.repository }}
```

**Security**:
The action downloads `.sha256` checksum files from releases and verifies archives before extraction. If verification fails, it falls back to building locally rather than using potentially compromised binaries.

---

## Design Principles

### 1. **Fail-Safe Defaults**
All actions gracefully handle failures and fall back to safe alternatives:
- Missing releases → build locally
- Failed downloads → build locally
- Checksum mismatches → build locally

### 2. **Platform Independence**
Actions work across Linux, macOS, and Windows using:
- Bash for Unix (including Windows Git Bash)
- PowerShell for Windows-specific operations
- Platform-specific tools (sha256sum, shasum, Get-FileHash)

### 3. **Performance Optimization**
- Aggressive caching (Rust toolchain, cross binary, cargo artifacts)
- Parallel job execution where possible
- Smart cache keys for better hit rates

### 4. **Security First**
- Checksum verification for downloaded artifacts
- Locked dependency versions (`cargo install --locked`)
- No secrets in logs

### 5. **Developer Experience**
- Clear, descriptive action names
- Comprehensive inline documentation
- Usage examples in action definitions
- Meaningful error messages

## Maintenance

### Updating cross Version
When updating the `cross` version, update the cache key in `setup-cross-compilation/action.yml`:
```yaml
key: ${{ runner.os }}-cross-v0.2.5  # Update version here
```

### Adding New Platforms
To add support for a new platform:
1. Add target to build matrix in workflows
2. Update `build-rust-library` action with platform-specific library names
3. Update `get-rust-library` action with platform-specific download logic

### Cache Management
Caches are scoped by:
- **Rust toolchain**: `${{ runner.os }}-cargo-${{ cache-key-suffix }}-${{ hashFiles('**/Cargo.lock') }}`
- **cross binary**: `${{ runner.os }}-cross-v0.2.5`

To invalidate caches, update the version suffix in cache keys.

## Testing

These actions are tested implicitly through the main workflows:
- `rust-ci.yml`: Tests cross-compilation for all platforms
- `go-ci.yml`: Tests library download and local build fallback
- `rust-release.yml`: Tests release artifact creation

## Contributing

When modifying composite actions:
1. Update this README with any new features or changes
2. Add usage examples for complex inputs
3. Ensure YAML syntax is valid (`yq eval 'keys' action.yml`)
4. Test in a PR to verify all workflows still pass
5. Update version numbers in cache keys if behavior changes

## Related Documentation

- [CLAUDE.md](../../CLAUDE.md) - Overall project architecture
- [Workflow Documentation](../workflows/) - Individual workflow details
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
