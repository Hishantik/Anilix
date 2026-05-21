#!/bin/sh
set -eu

# Anilix Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/hishantik/anilix/main/install.sh | sh
#   or:  sh install.sh [--update] [--uninstall] [--version v1.0.0] [--help]

REPO="hishantik/anilix"
BINARY="anilix"
BASE_URL="https://github.com/${REPO}/releases/download"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"

# --- Colors (disabled if not a terminal) ---
if [ -t 1 ]; then
    ESC=$(printf '\033')
    RED="${ESC}[0;31m"
    GREEN="${ESC}[0;32m"
    YELLOW="${ESC}[0;33m"
    BLUE="${ESC}[0;34m"
    BOLD="${ESC}[1m"
    RESET="${ESC}[0m"
else
    RED='' GREEN='' YELLOW='' BLUE='' BOLD='' RESET=''
fi

# --- Helpers ---
info()  { printf "%s[info]%s %s\n" "$BLUE" "$RESET" "$*"; }
ok()    { printf "%s[ok]%s %s\n" "$GREEN" "$RESET" "$*"; }
warn()  { printf "%s[warn]%s %s\n" "$YELLOW" "$RESET" "$*"; }
die()   { printf "%s[error]%s %s\n" "$RED" "$RESET" "$*" >&2; exit 1; }

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "Required command '$1' not found. Please install it first."
}

# --- Download with progress bar ---
draw_bar() {
    pct="$1"
    [ "$pct" -gt 100 ] && pct=100
    width=50
    on=$(( pct * width / 100 ))
    off=$(( width - on ))
    filled=$(printf "%*s" "$on" "")
    filled=${filled// /■}
    empty=$(printf "%*s" "$off" "")
    empty=${empty// /･}
    printf "\r\033[38;2;157;78;221m%s%s %3d%%\033[0m" "$filled" "$empty" "$pct" >&2
}

download_file() {
    url="$1"
    output="$2"

    # Start download in background
    curl -fSL "$url" -o "$output" -# 2>/dev/null &
    curl_pid=$!

    # Animate progress bar while downloading
    pct=0
    while kill -0 "$curl_pid" 2>/dev/null; do
        draw_bar "$pct"
        # Ramp up slowly: +1 per tick early on, accelerating
        if [ "$pct" -lt 30 ]; then
            pct=$((pct + 1))
        elif [ "$pct" -lt 60 ]; then
            pct=$((pct + 2))
        elif [ "$pct" -lt 85 ]; then
            pct=$((pct + 1))
        else
            # Slow crawl near end — wait for curl to finish
            pct=$((pct + 1))
        fi
        sleep 0.15
    done

    # Download finished — complete the bar
    draw_bar 100
    printf "\n" >&2

    wait "$curl_pid"
    return $?
}

# --- Cleanup trap ---
TMPDIR=""
cleanup() {
    [ -n "$TMPDIR" ] && rm -rf "$TMPDIR"
}
trap cleanup EXIT

# --- Detect OS ---
detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux*)     echo "linux" ;;
        Darwin*)    echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*) echo "windows" ;;
        *)          die "Unsupported OS: $os" ;;
    esac
}

# --- Detect Arch ---
detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)   echo "arm64" ;;
        *)               die "Unsupported architecture: $arch" ;;
    esac
}

# --- Detect Termux ---
is_termux() {
    case "${PREFIX:-}" in *com.termux*) return 0;; esac
    [ -n "${TERMUX_VERSION:-}" ] && return 0
    return 1
}

# --- Detect install directory ---
detect_install_dir() {
    if is_termux; then
        echo "${PREFIX}/bin"
        return
    fi

    # Try /usr/local/bin first
    if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
        echo "/usr/local/bin"
        return
    fi

    # Try to create /usr/local/bin (may need sudo)
    if [ -d /usr/local ]; then
        if [ -w /usr/local ]; then
            mkdir -p /usr/local/bin
            echo "/usr/local/bin"
            return
        fi
        # Try with sudo
        if command -v sudo >/dev/null 2>&1; then
            sudo mkdir -p /usr/local/bin 2>/dev/null && \
            sudo test -w /usr/local/bin && \
            echo "/usr/local/bin" && return
        fi
    fi

    # Fallback to ~/.local/bin
    mkdir -p "$HOME/.local/bin"
    echo "$HOME/.local/bin"
}

# --- Get latest version from GitHub API ---
get_latest_version() {
    need_cmd curl
    json=$(curl -fsSL "$API_URL")
    if command -v jq >/dev/null 2>&1; then
        tag=$(printf '%s' "$json" | jq -r '.tag_name // empty')
    else
        tag=$(printf '%s' "$json" | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')
    fi
    [ -z "$tag" ] && die "Failed to fetch latest version from GitHub"
    echo "$tag"
}

# --- Download and install binary ---
do_install() {
    version="${1:-}"
    os=$(detect_os)
    arch=$(detect_arch)
    install_dir=$(detect_install_dir)

    if [ -z "$version" ]; then
        info "Fetching latest version..."
        version=$(get_latest_version)
    fi

    info "Installing ${BOLD}${version}${RESET} for ${os}/${arch}..."

    # Build download URL
    if [ "$os" = "windows" ]; then
        ext="zip"
        archive_name="${BINARY}_${os}_${arch}.${ext}"
        url="${BASE_URL}/${version}/${archive_name}"
    elif is_termux; then
        ext="tar.gz"
        archive_name="${BINARY}_termux_${arch}.${ext}"
        url="${BASE_URL}/${version}/${archive_name}"
    else
        ext="tar.gz"
        archive_name="${BINARY}_${os}_${arch}.${ext}"
        url="${BASE_URL}/${version}/${archive_name}"
    fi

    # Create temp directory
    TMPDIR=$(mktemp -d)

    info "Downloading ${url}..."
    if ! download_file "$url" "${TMPDIR}/${archive_name}"; then
        die "Download failed. Check if version ${version} exists: https://github.com/${REPO}/releases"
    fi

    # Extract
    info "Extracting..."
    orig_dir=$(pwd)
    cd "$TMPDIR"
    if [ "$os" = "windows" ]; then
        need_cmd unzip
        unzip -qo "$archive_name"
    else
        tar -xzf "$archive_name"
    fi
    cd "$orig_dir"

    # Find binary
    if [ ! -f "${TMPDIR}/${BINARY}" ]; then
        die "Binary '${BINARY}' not found in archive"
    fi

    # Install
    chmod +x "${TMPDIR}/${BINARY}"
    if [ -w "$install_dir" ]; then
        cp "${TMPDIR}/${BINARY}" "${install_dir}/${BINARY}"
    elif command -v sudo >/dev/null 2>&1; then
        sudo cp "${TMPDIR}/${BINARY}" "${install_dir}/${BINARY}"
    else
        die "No write permission to ${install_dir} and sudo not available"
    fi

    ok "Installed ${BOLD}${install_dir}/${BINARY}${RESET}"

    # Check if install dir is in PATH
    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *)
            warn "${install_dir} is not in your PATH."
            warn "Add it by running:"
            printf "    ${BOLD}export PATH=\"${install_dir}:\$PATH\"${RESET}\n"
            warn "Add that line to your shell profile (~/.bashrc, ~/.zshrc, etc.)"
            ;;
    esac

    # Dependency check
    check_deps "$os"
}

# --- Check dependencies ---
check_deps() {
    os="${1:-$(detect_os)}"
    has_player=false

    if command -v mpv >/dev/null 2>&1; then
        has_player=true
    elif command -v vlc >/dev/null 2>&1; then
        has_player=true
    elif command -v iina >/dev/null 2>&1; then
        has_player=true
    fi

    if [ "$has_player" = false ]; then
        warn "No supported video player found. Anilix requires one of: mpv, vlc, iina"
        printf "\n  Install suggestions:\n"
        if is_termux; then
            printf "    ${BOLD}pkg install mpv${RESET}\n"
        elif [ "$os" = "darwin" ]; then
            printf "    ${BOLD}brew install mpv${RESET}      # or\n"
            printf "    ${BOLD}brew install --cask iina${RESET}\n"
        elif [ "$os" = "windows" ]; then
            printf "    Download mpv from: https://mpv.io/installation/\n"
        elif command -v apt >/dev/null 2>&1; then
            printf "    ${BOLD}sudo apt install mpv${RESET}\n"
        elif command -v dnf >/dev/null 2>&1; then
            printf "    ${BOLD}sudo dnf install mpv${RESET}\n"
        elif command -v pacman >/dev/null 2>&1; then
            printf "    ${BOLD}sudo pacman -S mpv${RESET}\n"
        elif command -v apk >/dev/null 2>&1; then
            printf "    ${BOLD}sudo apk add mpv${RESET}\n"
        fi
        printf "\n"
    fi
}

# --- Update ---
do_update() {
    info "Updating ${BINARY}..."
    do_install ""
}

# --- Uninstall ---
do_uninstall() {
    install_dir=$(detect_install_dir)
    binary_path="${install_dir}/${BINARY}"

    if [ ! -f "$binary_path" ]; then
        die "${BINARY} not found at ${binary_path}"
    fi

    rm -f "$binary_path"
    ok "Removed ${binary_path}"
}

# --- Usage ---
usage() {
    cat <<EOF
${BOLD}Anilix Installer${RESET}

Usage:
  install.sh [options]

Options:
  (none)           Install the latest version
  --update         Update to the latest version
  --uninstall      Remove the installed binary
  --version VER    Install a specific version (e.g. --version v1.0.0)
  --help           Show this help message

Examples:
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh
  sh install.sh --version v1.0.0
  sh install.sh --update
  sh install.sh --uninstall
EOF
}

# --- Main ---
main() {
    # If piped (curl | sh), default to install
    # If called with args, parse them
    action="install"
    version=""

    while [ $# -gt 0 ]; do
        case "$1" in
            --help|-h)
                usage
                exit 0
                ;;
            --update|-u)
                action="update"
                shift
                ;;
            --uninstall)
                action="uninstall"
                shift
                ;;
            --version|-v)
                [ $# -lt 2 ] && die "--version requires a value (e.g. --version v1.0.0)"
                version="$2"
                action="install"
                shift 2
                ;;
            *)
                die "Unknown option: $1. Run with --help for usage."
                ;;
        esac
    done

    case "$action" in
        install)   do_install "$version" ;;
        update)    do_update ;;
        uninstall) do_uninstall ;;
    esac
}

main "$@"
