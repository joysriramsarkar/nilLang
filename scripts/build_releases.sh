#!/usr/bin/env bash
# Build release archives for all supported platforms
set -e

VERSION="0.1.0"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${REPO_ROOT}/dist"

mkdir -p "${DIST_DIR}"
cd "${REPO_ROOT}"

PLATFORMS=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64")
COMMANDS=("nil" "nilc" "nilpkg" "nilpkg-server" "nilkey" "softbusd")

echo "🚀 Building Nilang v${VERSION} release archives..."

for PLATFORM in "${PLATFORMS[@]}"; do
    OS="${PLATFORM%/*}"
    ARCH="${PLATFORM#*/}"
    EXT=""
    if [ "$OS" = "windows" ]; then
        EXT=".exe"
    fi

    TARGET_DIR="${DIST_DIR}/nilang-v${VERSION}-${OS}-${ARCH}"
    mkdir -p "${TARGET_DIR}"

    echo "📦 Building for ${OS}/${ARCH}..."
    for CMD in "${COMMANDS[@]}"; do
        GOOS="${OS}" GOARCH="${ARCH}" CGO_ENABLED=0 go build -o "${TARGET_DIR}/${CMD}${EXT}" "./cmd/${CMD}"
    done

    cp README.md LICENSE "${TARGET_DIR}/"

    if [ "$OS" = "windows" ]; then
        cd "${DIST_DIR}"
        zip -rq "nilang-v${VERSION}-${OS}-${ARCH}.zip" "nilang-v${VERSION}-${OS}-${ARCH}"
        rm -rf "${TARGET_DIR}"
        cd "${REPO_ROOT}"
    else
        tar -czf "${DIST_DIR}/nilang-v${VERSION}-${OS}-${ARCH}.tar.gz" -C "${DIST_DIR}" "nilang-v${VERSION}-${OS}-${ARCH}"
        rm -rf "${TARGET_DIR}"
    fi
done

cd "${DIST_DIR}"
sha256sum nilang-v* > SHA256SUMS
echo "✅ Releases generated in ${DIST_DIR}/ with SHA256SUMS"
