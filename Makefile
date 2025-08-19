# Build targets
.PHONY: build
build:
	cargo zigbuild

.PHONY: build-debug
build-debug:
	cargo build

.PHONY: build-all-targets
build-all-targets:
	CFLAGS="-I/opt/homebrew/opt/libiconv/include" CXXFLAGS="-I/opt/homebrew/opt/libiconv/include" RUSTFLAGS="-L/opt/homebrew/opt/libiconv/lib -C link-arg=-L/opt/homebrew/opt/libiconv/lib" cargo zigbuild --release --target x86_64-unknown-linux-gnu
	CFLAGS="-I/opt/homebrew/opt/libiconv/include" CXXFLAGS="-I/opt/homebrew/opt/libiconv/include" RUSTFLAGS="-L/opt/homebrew/opt/libiconv/lib -C link-arg=-L/opt/homebrew/opt/libiconv/lib" cargo zigbuild --release --target aarch64-unknown-linux-gnu
	#CFLAGS="-I/opt/homebrew/opt/libiconv/include" CXXFLAGS="-I/opt/homebrew/opt/libiconv/include" RUSTFLAGS="-L/opt/homebrew/opt/libiconv/lib -C link-arg=-L/opt/homebrew/opt/libiconv/lib" cargo zigbuild --release --target x86_64-apple-darwin
	CFLAGS="-I/opt/homebrew/opt/libiconv/include" CXXFLAGS="-I/opt/homebrew/opt/libiconv/include" RUSTFLAGS="-L/opt/homebrew/opt/libiconv/lib -C link-arg=-L/opt/homebrew/opt/libiconv/lib" cargo zigbuild --release --target aarch64-apple-darwin
	CFLAGS="-I/opt/homebrew/opt/libiconv/include" CXXFLAGS="-I/opt/homebrew/opt/libiconv/include" RUSTFLAGS="-L/opt/homebrew/opt/libiconv/lib -C link-arg=-L/opt/homebrew/opt/libiconv/lib" cargo zigbuild --release --target x86_64-pc-windows-msvc

# Test targets
.PHONY: gotestsum-bin
gotestsum-bin:
	go install gotest.tools/gotestsum@latest

.PHONY: test
test: gotestsum-bin build
	gotestsum \
        --format short-verbose \
        --rerun-fails=5 \
        --packages="./..." \
        --junitfile unit.xml \
        -- \
        -v \
        -coverprofile=coverage.out \
        -timeout=30m

# this is meant to run in CI environments where the library path is set up correctly
.PHONY: test-ci
test-ci: gotestsum-bin
	gotestsum \
        --format short-verbose \
        --rerun-fails=5 \
        --packages="./..." \
        --junitfile unit.xml \
        -- \
        -v \
        -coverprofile=coverage.out \
        -timeout=30m

.PHONY: test-lib-path
test-lib-path: gotestsum-bin build
	TOKENIZERS_LIB_PATH="$(shell pwd)/target/release/libtokenizers$(shell if [ "$(shell uname)" = "Darwin" ]; then echo ".dylib"; elif [ "$(shell uname)" = "Linux" ]; then echo ".so"; else echo ".dll"; fi)" \
	gotestsum \
        --format short-verbose \
        --rerun-fails=5 \
        --packages="./..." \
        --junitfile unit.xml \
        -- \
        -v \
        -coverprofile=coverage.out \
        -timeout=30m

.PHONY: test-rust
test-rust:
	cargo test --verbose

.PHONY: test-download
test-download: build
	TOKENIZERS_LIB_PATH="$(shell pwd)/target/release/libtokenizers$(shell if [ "$(shell uname)" = "Darwin" ]; then echo ".dylib"; elif [ "$(shell uname)" = "Linux" ]; then echo ".so"; else echo ".dll"; fi)" \
	go test -v -run "TestDownloadFunctionality|TestGetLibraryInfo"

# Lint targets
.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...

.PHONY: lint-rust
lint-rust:
	cargo clippy -- -D warnings
	cargo fmt -- --check

.PHONY: fmt-rust
fmt-rust:
	cargo fmt

# Release targets
.PHONY: create-release-assets
create-release-assets: build-all-targets
	mkdir -p release-assets
	# Linux x86_64
	tar -czf release-assets/libtokenizers-x86_64-unknown-linux-gnu.tar.gz -C target/x86_64-unknown-linux-gnu/release libtokenizers.so
	sha256sum release-assets/libtokenizers-x86_64-unknown-linux-gnu.tar.gz > release-assets/libtokenizers-x86_64-unknown-linux-gnu.tar.gz.sha256
	# Linux aarch64
	tar -czf release-assets/libtokenizers-aarch64-unknown-linux-gnu.tar.gz -C target/aarch64-unknown-linux-gnu/release libtokenizers.so
	sha256sum release-assets/libtokenizers-aarch64-unknown-linux-gnu.tar.gz > release-assets/libtokenizers-aarch64-unknown-linux-gnu.tar.gz.sha256
	# macOS x86_64
	tar -czf release-assets/libtokenizers-x86_64-apple-darwin.tar.gz -C target/x86_64-apple-darwin/release libtokenizers.dylib
	sha256sum release-assets/libtokenizers-x86_64-apple-darwin.tar.gz > release-assets/libtokenizers-x86_64-apple-darwin.tar.gz.sha256
	# macOS aarch64
	tar -czf release-assets/libtokenizers-aarch64-apple-darwin.tar.gz -C target/aarch64-apple-darwin/release libtokenizers.dylib
	sha256sum release-assets/libtokenizers-aarch64-apple-darwin.tar.gz > release-assets/libtokenizers-aarch64-apple-darwin.tar.gz.sha256
	# Windows x86_64
	tar -czf release-assets/libtokenizers-x86_64-pc-windows-msvc.tar.gz -C target/x86_64-pc-windows-msvc/release libtokenizers.dll
	sha256sum release-assets/libtokenizers-x86_64-pc-windows-msvc.tar.gz > release-assets/libtokenizers-x86_64-pc-windows-msvc.tar.gz.sha256

.PHONY: clean
clean:
	cargo clean
	rm -rf release-assets
	go clean -testcache

# Development helpers
.PHONY: dev-setup
dev-setup:
	rustup target add x86_64-unknown-linux-gnu
	rustup target add aarch64-unknown-linux-gnu
	rustup target add x86_64-apple-darwin
	rustup target add aarch64-apple-darwin
	rustup target add x86_64-pc-windows-msvc
	rustup component add rustfmt clippy
	go install gotest.tools/gotestsum@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: check-env
check-env:
	@echo "Rust version: $(shell rustc --version)"
	@echo "Cargo version: $(shell cargo --version)"
	@echo "Go version: $(shell go version)"
	@echo "Platform: $(shell uname -s)-$(shell uname -m)"
	@echo "Library extension: $(shell if [ "$(shell uname)" = "Darwin" ]; then echo ".dylib"; elif [ "$(shell uname)" = "Linux" ]; then echo ".so"; else echo ".dll"; fi)"