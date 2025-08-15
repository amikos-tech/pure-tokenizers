#!/bin/bash

# Release preparation script
set -e

VERSION=${1:-}
if [ -z "$VERSION" ]; then
    echo "❌ Usage: $0 <version>"
    echo "   Example: $0 v1.0.0"
    exit 1
fi

echo "🚀 Preparing release $VERSION..."

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    echo "❌ Invalid version format. Use semantic versioning (e.g., v1.0.0)"
    exit 1
fi

# Check if we're on main/master branch
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$BRANCH" != "main" && "$BRANCH" != "master" ]]; then
    echo "⚠️  Warning: Not on main/master branch (current: $BRANCH)"
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "❌ Uncommitted changes detected. Please commit or stash them first."
    exit 1
fi

# Update Cargo.toml version (remove 'v' prefix)
CARGO_VERSION=${VERSION#v}
echo "📝 Updating Cargo.toml version to $CARGO_VERSION..."
sed -i.bak "s/^version = \".*\"/version = \"$CARGO_VERSION\"/" Cargo.toml
rm Cargo.toml.bak

# Update go.mod if needed (Go modules don't typically need version updates)

# Run tests to ensure everything works
echo "🧪 Running tests..."
make test-rust
echo "✅ Rust tests passed!"

# Build for current platform to verify
echo "🔧 Building for current platform..."
cargo build --release
echo "✅ Build successful!"

# Commit version changes
echo "📝 Committing version changes..."
git add Cargo.toml
git commit -m "chore: bump version to $VERSION"

# Create and push tag
echo "🏷️  Creating tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "✅ Release $VERSION prepared!"
echo ""
echo "Next steps:"
echo "1. Push changes: git push origin $BRANCH"
echo "2. Push tag: git push origin $VERSION"
echo "3. GitHub Actions will automatically build and create the release"
echo ""
echo "Or push both at once:"
echo "git push origin $BRANCH && git push origin $VERSION"