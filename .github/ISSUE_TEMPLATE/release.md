---
name: Release Request
about: Request a new release
title: 'Release v[VERSION]'
labels: 'release'
assignees: ''
---

## Release Information

**Version:** v[VERSION] (e.g., v1.0.0)

**Release Type:**
- [ ] Major release (breaking changes)
- [ ] Minor release (new features)
- [ ] Patch release (bug fixes)

## Pre-release Checklist

- [ ] All tests are passing
- [ ] Documentation is updated
- [ ] CHANGELOG is updated (if applicable)
- [ ] Version is updated in Cargo.toml
- [ ] Cross-platform builds are working

## Changes Since Last Release

<!-- Describe the main changes, new features, bug fixes, etc. -->

## Breaking Changes

<!-- List any breaking changes (for major releases) -->

## Additional Notes

<!-- Any additional information about this release -->

---

**Release Process:**
1. Run `./scripts/prepare-release.sh v[VERSION]`
2. Push changes and tag
3. GitHub Actions will automatically create the release