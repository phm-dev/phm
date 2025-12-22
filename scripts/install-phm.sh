#!/bin/bash
set -e

# PHM Installer Script
# Usage: curl -fsSL https://raw.githubusercontent.com/phm-dev/phm/main/scripts/install-phm.sh | bash

REPO="phm-dev/phm"
INSTALL_DIR="/usr/local/bin"
PHM_DATA_DIR="/var/phm"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}==>${NC} $1"
}

success() {
    echo -e "${GREEN}==>${NC} $1"
}

warn() {
    echo -e "${YELLOW}Warning:${NC} $1"
}

error() {
    echo -e "${RED}Error:${NC} $1"
    exit 1
}

# Check if running on macOS
check_os() {
    if [[ "$(uname)" != "Darwin" ]]; then
        error "PHM is only supported on macOS"
    fi
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64)
            echo "amd64"
            ;;
        arm64|aarch64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $arch"
            ;;
    esac
}

# Get latest version from GitHub
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$version" ]]; then
        error "Failed to get latest version"
    fi
    echo "$version"
}

# Download and install
install_phm() {
    local version="$1"
    local arch="$2"
    local download_url="https://github.com/${REPO}/releases/download/${version}/phm-${version}-darwin-${arch}.tar.gz"
    local tmp_dir
    tmp_dir=$(mktemp -d)

    info "Downloading PHM ${version} for darwin-${arch}..."
    if ! curl -fsSL "$download_url" -o "${tmp_dir}/phm.tar.gz"; then
        rm -rf "$tmp_dir"
        error "Failed to download PHM"
    fi

    info "Extracting..."
    tar -xzf "${tmp_dir}/phm.tar.gz" -C "$tmp_dir"

    info "Installing to ${INSTALL_DIR}..."
    # Create install directory if it doesn't exist
    if [[ ! -d "$INSTALL_DIR" ]]; then
        sudo mkdir -p "$INSTALL_DIR"
    fi

    if [[ -w "$INSTALL_DIR" ]]; then
        mv "${tmp_dir}/phm" "${INSTALL_DIR}/phm"
        chmod +x "${INSTALL_DIR}/phm"
    else
        sudo mv "${tmp_dir}/phm" "${INSTALL_DIR}/phm"
        sudo chmod +x "${INSTALL_DIR}/phm"
    fi

    # Create data directories
    info "Creating data directories..."
    sudo mkdir -p "${PHM_DATA_DIR}/installed"
    sudo mkdir -p "${PHM_DATA_DIR}/cache"
    sudo mkdir -p "/opt/php/bin"

    # Cleanup
    rm -rf "$tmp_dir"

    success "PHM ${version} installed successfully!"
}

# Check if phm is already installed
check_existing() {
    if command -v phm &> /dev/null; then
        local current_version
        current_version=$(phm --version 2>/dev/null | awk '{print $3}')
        warn "PHM is already installed (version: ${current_version})"
        read -p "Do you want to upgrade? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 0
        fi
    fi
}

# Print post-install instructions
print_instructions() {
    echo ""
    echo -e "${GREEN}Installation complete!${NC}"
    echo ""
    echo "To use PHM-managed PHP versions, add to your shell profile:"
    echo ""
    echo -e "  ${YELLOW}export PATH=\"/opt/php/bin:\$PATH\"${NC}"
    echo ""
    echo "Quick start:"
    echo "  phm install php8.5-cli  # Install PHP 8.5 CLI"
    echo "  phm use 8.5             # Set PHP 8.5 as default"
    echo "  phm list -a             # List available packages"
    echo "  phm ui                  # Interactive mode"
    echo ""
}

main() {
    echo ""
    echo -e "${BLUE}PHM Installer${NC}"
    echo "============="
    echo ""

    check_os
    check_existing

    local arch
    arch=$(detect_arch)
    info "Detected architecture: ${arch}"

    local version
    version=$(get_latest_version)
    info "Latest version: ${version}"

    install_phm "$version" "$arch"
    print_instructions
}

main "$@"
