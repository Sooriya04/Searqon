#!/usr/bin/env sh
# Searqon One-Line Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Sooriya04/Searqon/main/install.sh | sh

set -e

REPO="Sooriya04/Searqon"
DEFAULT_VERSION="v1.2.1"

# ANSI Colors (POSIX compliant)
BOLD="$(printf '\033[1m')"
GREEN="$(printf '\033[32m')"
CYAN="$(printf '\033[36m')"
YELLOW="$(printf '\033[33m')"
RED="$(printf '\033[31m')"
RESET="$(printf '\033[0m')"

info() {
    printf "${CYAN}→${RESET} %s\n" "$1"
}

success() {
    printf "${GREEN}✓${RESET} ${BOLD}%s${RESET}\n" "$1"
}

warn() {
    printf "${YELLOW}!${RESET} %s\n" "$1"
}

error() {
    printf "${RED}✗ Error:${RESET} %s\n" "$1" >&2
    exit 1
}

# 1. Detect OS
OS_RAW="$(uname -s)"
case "${OS_RAW}" in
    Linux*)  OS="linux" ;;
    Darwin*) OS="darwin" ;;
    *)       error "Unsupported operating system: ${OS_RAW}. Searqon installer supports Linux and macOS." ;;
esac

# 2. Detect Architecture
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)              error "Unsupported CPU architecture: ${ARCH_RAW}. Searqon supports amd64 (x86_64) and arm64." ;;
esac

info "Detected platform: ${BOLD}${OS}/${ARCH}${RESET}"

# 3. Determine Version
VERSION="${SEARQON_VERSION:-}"
if [ -z "${VERSION}" ]; then
    # Try fetching latest release tag from GitHub API
    if command -v curl >/dev/null 2>&1; then
        LATEST=$(curl -fsSL -H "User-Agent: searqon-installer" "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' || true)
    fi
    VERSION="${LATEST:-$DEFAULT_VERSION}"
fi

# Ensure version has 'v' prefix for release tag
case "${VERSION}" in
    v*) ;;
    *)  VERSION="v${VERSION}" ;;
esac

info "Target version: ${BOLD}${VERSION}${RESET}"

# 4. Construct asset name & download URL
ASSET_NAME="searqon-${VERSION}-${OS}-${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"

TMP_DIR="$(mktemp -d -t searqon-install-XXXXXX)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT INT TERM

info "Downloading ${ASSET_NAME}..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL --progress-bar "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ASSET_NAME}"
elif command -v wget >/dev/null 2>&1; then
    wget -q --show-progress "${DOWNLOAD_URL}" -O "${TMP_DIR}/${ASSET_NAME}"
else
    error "Neither curl nor wget found in PATH. Please install either to continue."
fi

# 5. Extract binary
info "Extracting ${ASSET_NAME}..."
tar -xzf "${TMP_DIR}/${ASSET_NAME}" -C "${TMP_DIR}"

BIN_EXTRACTED="${TMP_DIR}/searqon-${OS}-${ARCH}"
if [ ! -f "${BIN_EXTRACTED}" ]; then
    BIN_EXTRACTED="$(find "${TMP_DIR}" -type f -name 'searqon*' | head -n 1)"
fi

if [ ! -f "${BIN_EXTRACTED}" ]; then
    error "Failed to locate extracted searqon binary."
fi

chmod +x "${BIN_EXTRACTED}"

# 6. Determine Install Destination
INSTALL_DIR="/usr/local/bin"
USE_SUDO=0

if [ -w "${INSTALL_DIR}" ]; then
    DEST="${INSTALL_DIR}/searqon"
elif [ "$(id -u)" -eq 0 ]; then
    DEST="${INSTALL_DIR}/searqon"
elif sudo -n true 2>/dev/null; then
    USE_SUDO=1
    DEST="${INSTALL_DIR}/searqon"
else
    # Fallback to user-local directory (~/.local/bin)
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "${INSTALL_DIR}"
    DEST="${INSTALL_DIR}/searqon"
fi

info "Installing to ${BOLD}${DEST}${RESET}..."
if [ "${USE_SUDO}" -eq 1 ]; then
    sudo cp "${BIN_EXTRACTED}" "${DEST}"
    sudo chmod 755 "${DEST}"
else
    cp "${BIN_EXTRACTED}" "${DEST}"
    chmod 755 "${DEST}"
fi

success "Searqon ${VERSION} installed successfully at ${DEST}!"

# 7. Check PATH
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
        warn "${INSTALL_DIR} is not in your current PATH."
        warn "Add it by running:"
        printf "    export PATH=\"%s:\$PATH\"\n\n" "${INSTALL_DIR}"
        ;;
esac

printf "\n${BOLD}Quick Start:${RESET}\n"
printf "  • Start HTTP & UTCP server : ${CYAN}searqon${RESET}\n"
printf "  • Launch MCP stdio server  : ${CYAN}searqon mcp${RESET}\n"
printf "  • Check version            : ${CYAN}searqon version${RESET}\n\n"
