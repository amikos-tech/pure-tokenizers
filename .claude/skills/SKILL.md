---
description: Automated release process for pure-tokenizers (Rust + Go dual release) with user confirmation
---

You are helping create a new release for the pure-tokenizers project. This project requires coordinated releases of both Rust library binaries and the Go module.

## Your Task

Execute the complete release process, handling all steps from version bump through final verification. The process is automated but requires explicit user confirmation before executing any git operations (commits, pushes, tags).

## Release Process Steps

### 1. Initial Setup and Validation

First, use the TodoWrite tool to create a comprehensive task list for the entire release process:

```
- Validate prerequisites and git state
- Get version number from user
- Update Cargo.toml with new version
- Get user confirmation for git operations
- Commit and push version bump
- Create and push Rust release tag
- Monitor Rust release workflow
- Verify Rust release artifacts
- Create and push Go release tag
- Monitor Go release workflow
- Handle Go release failures if needed
- Verify final releases
```

Then validate:
- Check `git status` shows clean working directory
- Check current branch is `main`
- Verify `gh` CLI is available and authenticated

If any validation fails, stop and inform the user what needs to be fixed.

### 2. Version Selection

Ask the user what version they want to release using AskUserQuestion. Provide options like:
- Patch bump (bug fixes only)
- Minor bump (new features, no breaking changes)
- Major bump (breaking changes)
- Custom version

Also show them the current version from `Cargo.toml` and recent commits since last release to help them decide.

### 3. Update Cargo.toml

- Read `Cargo.toml`
- Update the version field to the new version
- Use the Edit tool to make the change
- Show the user the diff/change that was made
- Mark todo as completed

### 4. Get User Confirmation for Git Operations

**IMPORTANT: Before executing ANY git commands, you must get explicit user confirmation.**

Show the user a summary of what will happen:
```
Ready to proceed with release vX.Y.Z:

Changes to commit:
- Cargo.toml: version updated to X.Y.Z

Git operations that will be performed:
1. git add Cargo.toml
2. git commit -m "chore: bump version to X.Y.Z"
3. git push
4. git tag rust-vX.Y.Z
5. git push origin rust-vX.Y.Z
6. git tag vX.Y.Z
7. git push origin vX.Y.Z

This will trigger CI/CD workflows and create public releases.
```

Use AskUserQuestion to ask:
- Question: "Do you want to proceed with these git operations?"
- Options: "Yes, proceed with release" / "No, cancel"

If user selects "No, cancel":
- Stop the release process
- Inform user that they can manually review changes and restart when ready

If user selects "Yes, proceed with release":
- Continue to the next step

### 5. Commit and Push Version Bump

Execute:
```bash
git add Cargo.toml && git commit -m "chore: bump version to X.Y.Z" && git push
```

Mark todo as completed.

### 6. Create Rust Release

Execute:
```bash
git tag rust-vX.Y.Z && git push origin rust-vX.Y.Z
```

Inform user: "Creating Rust release rust-vX.Y.Z..."
Mark todo as completed.

### 7. Monitor Rust Release Workflow

- Use `gh run list --limit 5` to find the "Rust Release" workflow run
- Extract the run ID
- Use `gh run watch <RUN_ID>` to monitor (with timeout of 600000ms / 10 minutes)
- Mark todo as completed when workflow succeeds

If the workflow fails, stop and inform the user about the failure with logs.

### 8. Verify Rust Release Artifacts

Execute:
```bash
gh release view rust-vX.Y.Z
```

Verify that all 7 platform artifacts are present:
- libtokenizers-aarch64-apple-darwin.tar.gz
- libtokenizers-aarch64-unknown-linux-gnu.tar.gz
- libtokenizers-aarch64-unknown-linux-musl.tar.gz
- libtokenizers-x86_64-apple-darwin.tar.gz
- libtokenizers-x86_64-pc-windows-msvc.tar.gz
- libtokenizers-x86_64-unknown-linux-gnu.tar.gz
- libtokenizers-x86_64-unknown-linux-musl.tar.gz

If any are missing, stop and inform the user.
Mark todo as completed.

### 9. Create Go Release

Execute:
```bash
git tag vX.Y.Z && git push origin vX.Y.Z
```

Inform user: "Creating Go release vX.Y.Z..."
Mark todo as completed.

### 10. Monitor Go Release Workflow

- Use `gh run list --limit 5` to find the "Go Release" workflow run
- Extract the run ID
- Use `gh run watch <RUN_ID>` to monitor (with timeout of 600000ms / 10 minutes)

**Important**: The Go release may fail if it started too quickly and couldn't download Rust artifacts yet.

If the workflow fails:
1. Check the failure reason using `gh run view <RUN_ID> --log-failed`
2. If the error contains "undefined symbol" (indicating timing issue):
   - Inform user: "Go release failed due to timing issue. Retrying automatically..."
   - Execute: `gh run rerun <RUN_ID> --failed`
   - Monitor the re-run with `gh run watch <RUN_ID>` again
   - If it succeeds on retry, mark todo as completed
3. If it fails for other reasons, stop and inform the user

Mark todo as completed when workflow succeeds.

### 11. Verify Final Releases

Execute both:
```bash
gh release view rust-vX.Y.Z
gh release view vX.Y.Z
```

Confirm:
- Rust release has all 7 artifacts
- Go release was created
- Both releases are published (not draft)

Mark todo as completed.

### 12. Success Summary

Provide a clear summary:

```
✓ Release vX.Y.Z completed successfully!

Rust Release: https://github.com/amikos-tech/pure-tokenizers/releases/tag/rust-vX.Y.Z
Go Release: https://github.com/amikos-tech/pure-tokenizers/releases/tag/vX.Y.Z

To install:
go get github.com/amikos-tech/pure-tokenizers@vX.Y.Z

All 7 platform binaries are available:
- Linux (x86_64 + aarch64, GNU + MUSL)
- macOS (x86_64 + Apple Silicon)
- Windows (x86_64)
```

## Important Notes

- **Always use TodoWrite** to track progress through all steps
- **Mark todos completed** immediately after finishing each step
- **Handle timing issues** - Go release may need retry if it starts too quickly
- **Stop on errors** - Don't proceed if validation or workflows fail
- **Be informative** - Tell the user what's happening at each step
- **Verify everything** - Check that artifacts exist before proceeding

## Error Handling

Common issues and solutions:

1. **"undefined symbol" in Go tests**: Timing issue, automatically retry the workflow
2. **Missing artifacts**: Stop and report - likely a build failure
3. **Workflow timeout**: Report to user with run ID so they can investigate
4. **Git not clean**: Stop immediately and tell user to commit or stash changes
5. **Not on main branch**: Stop and tell user to switch to main

## Example Flow

1. User invokes skill with `/release` or by asking to create a release
2. Skill creates comprehensive todo list
3. Skill validates prerequisites (git state, branch, tools)
4. Skill asks user for version number
5. Skill updates Cargo.toml and shows the change
6. **Skill asks for explicit confirmation before any git operations**
7. If confirmed, skill executes all git operations (commit, push, tags)
8. Skill monitors workflows, updating todos as it progresses
9. If Go release fails due to timing, skill automatically retries
10. Skill provides final success summary with URLs

The process is automated but safe - the user must explicitly confirm before any commits or pushes are made.
