#!/usr/bin/env bash
# Nilang (নীলাং) Universal Installer for Linux & macOS
# Powered by Alap Framework & Onuron OS

set -e

RESET='\033[0m'
BOLD='\033[1m'
GREEN='\033[32m'
BLUE='\033[34m'
RED='\033[31m'
YELLOW='\033[33m'

NIL_VERSION="0.1.0"
INSTALL_DIR="${HOME}/.nilang"
BIN_DIR="${INSTALL_DIR}/bin"
GITHUB_REPO="joysriramsarkar/nilLang"

echo -e "${BLUE}${BOLD}"
echo "╔══════════════════════════════════════════════════╗"
echo "║                                                  ║"
echo "║   ███╗   ██╗██╗██╗     █████╗ ███╗   ██╗ ███╗  ║"
echo "║   ████╗  ██║██║██║    ██╔══██╗████╗  ██║██╔══╝  ║"
echo "║   ██╔██╗ ██║██║██║    ███████║██╔██╗ ██║██║  ███║"
echo "║   ██║╚██╗██║██║██║    ██╔══██║██║╚██╗██║██║   ║"
echo "║   ██║ ╚████║██║██████╗██║  ██║██║ ╚████║╚██████║"
echo "║   ╚═╝  ╚═══╝╚═╝╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝"
echo "║                                                  ║"
echo "║   Nilang Installer for Linux & macOS             ║"
echo "║   Powered by Alap Framework • Onuron OS          ║"
echo "║                                                  ║"
echo "╚══════════════════════════════════════════════════╝"
echo -e "${RESET}"

# 1. Detect OS & Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux)
        TARGET_OS="linux"
        ;;
    darwin)
        TARGET_OS="darwin"
        ;;
    *)
        echo -e "${RED}❌ Unsupported operating system: ${OS}${RESET}"
        echo "Nilang supports Linux, macOS, and Onuron OS."
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        TARGET_ARCH="amd64"
        ;;
    arm64|aarch64)
        TARGET_ARCH="arm64"
        ;;
    *)
        echo -e "${RED}❌ Unsupported architecture: ${ARCH}${RESET}"
        exit 1
        ;;
esac

echo -e "🖥️  Detected System: ${GREEN}${TARGET_OS}-${TARGET_ARCH}${RESET}"

# 2. Create install directory
mkdir -p "${BIN_DIR}"

# 3. Check for local build or remote download
RELEASE_ARCHIVE="nilang-v${NIL_VERSION}-${TARGET_OS}-${TARGET_ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/v${NIL_VERSION}/${RELEASE_ARCHIVE}"

INSTALLED=0

if command -v go >/dev/null 2>&1; then
    echo -e "🔨 Building binaries using local Go toolchain..."
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -f "${SCRIPT_DIR}/go.mod" ]; then
        cd "${SCRIPT_DIR}"
        go build -o "${BIN_DIR}/nil" ./cmd/nil
        go build -o "${BIN_DIR}/nilc" ./cmd/nilc
        go build -o "${BIN_DIR}/nilpkg" ./cmd/nilpkg
        go build -o "${BIN_DIR}/nilpkg-server" ./cmd/nilpkg-server
        go build -o "${BIN_DIR}/nilkey" ./cmd/nilkey
        go build -o "${BIN_DIR}/softbusd" ./cmd/softbusd
        INSTALLED=1
    fi
fi

if [ "$INSTALLED" -eq 0 ]; then
    echo -e "🌐 Downloading prebuilt binaries from GitHub Releases..."
    TEMP_DIR="$(mktemp -d)"
    trap 'rm -rf "${TEMP_DIR}"' EXIT

    if curl -fsSL "${DOWNLOAD_URL}" -o "${TEMP_DIR}/${RELEASE_ARCHIVE}"; then
        tar -xzf "${TEMP_DIR}/${RELEASE_ARCHIVE}" -C "${BIN_DIR}"
        INSTALLED=1
    else
        echo -e "${YELLOW}⚠️ Prebuilt release not found online, compiling from source...${RESET}"
        if command -v go >/dev/null 2>&1; then
            git clone "https://github.com/${GITHUB_REPO}.git" "${TEMP_DIR}/nilang-src"
            cd "${TEMP_DIR}/nilang-src"
            go build -o "${BIN_DIR}/nil" ./cmd/nil
            go build -o "${BIN_DIR}/nilc" ./cmd/nilc
            go build -o "${BIN_DIR}/nilpkg" ./cmd/nilpkg
            go build -o "${BIN_DIR}/nilpkg-server" ./cmd/nilpkg-server
            go build -o "${BIN_DIR}/nilkey" ./cmd/nilkey
            go build -o "${BIN_DIR}/softbusd" ./cmd/softbusd
            INSTALLED=1
        else
            echo -e "${RED}❌ Go toolchain or network connection required to install Nilang.${RESET}"
            exit 1
        fi
    fi
fi

# Ensure executable permissions
chmod +x "${BIN_DIR}"/*

# 4. Configure PATH in Shell Profiles
SHELL_NAME="$(basename "${SHELL:-bash}")"
PROFILE_FILES=()

case "${SHELL_NAME}" in
    zsh)
        PROFILE_FILES=("${HOME}/.zshrc" "${HOME}/.zprofile")
        ;;
    bash)
        PROFILE_FILES=("${HOME}/.bashrc" "${HOME}/.bash_profile" "${HOME}/.profile")
        ;;
    *)
        PROFILE_FILES=("${HOME}/.profile")
        ;;
esac

PATH_STR="export PATH=\"${BIN_DIR}:\$PATH\""

for pfile in "${PROFILE_FILES[@]}"; do
    if [ -f "${pfile}" ]; then
        if ! grep -q "${BIN_DIR}" "${pfile}"; then
            echo "" >> "${pfile}"
            echo "# Nilang Programming Language" >> "${pfile}"
            echo "${PATH_STR}" >> "${pfile}"
            echo -e "📝 Added Nilang to PATH in: ${GREEN}${pfile}${RESET}"
        fi
    fi
done

echo ""
echo -e "${GREEN}══════════════════════════════════════════════════${RESET}"
echo -e "${GREEN}${BOLD}✅ Nilang v${NIL_VERSION} successfully installed!${RESET}"
echo -e "📂 Location: ${BIN_DIR}"
echo -e "🛠️  Binaries installed:"
echo -e "   - ${BOLD}nil${RESET}            (Main compiler, runner, REPL & UI renderer)"
echo -e "   - ${BOLD}nilc${RESET}           (Dedicated bytecode compiler & disassembler)"
echo -e "   - ${BOLD}nilpkg${RESET}         (Package & dependency manager)"
echo -e "   - ${BOLD}nilpkg-server${RESET}  (Package registry & dashboard server)"
echo -e "   - ${BOLD}nilkey${RESET}         (Ed25519 cryptographic key & signing tool)"
echo -e "   - ${BOLD}softbusd${RESET}       (Distributed Onuron SoftBus daemon)"
echo -e "${GREEN}══════════════════════════════════════════════════${RESET}"
echo ""
echo -e "👉 To start using Nilang right away in this terminal:"
echo -e "   ${BOLD}export PATH=\"${BIN_DIR}:\$PATH\"${RESET}"
echo ""
echo -e "👉 Try running:"
echo -e "   ${BOLD}nil version${RESET}"
echo -e "   ${BOLD}nil repl${RESET}"
echo ""
