#!/usr/bin/env bash
# Update Bootstrap Icons SVG sprite for the Web UI
# Usage: ./update-bootstrap-icons.sh [version]
#
# If no version is specified, the script fetches the latest release.
# Examples:
#   ./update-bootstrap-icons.sh        # Get latest version
#   ./update-bootstrap-icons.sh 1.11.3 # Get specific version

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Target path for the sprite
TARGET_PATH="webui/cmd/webui/web/static/vendor/bootstrap-icons/bootstrap-icons.svg"
TEMP_DIR=$(mktemp -d)

# Cleanup on exit
cleanup() {
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Print colored message
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

# Get latest version from GitHub API
get_latest_version() {
    local version=$(curl -sSL https://api.github.com/repos/twbs/icons/releases/latest | grep '"tag_name":' | sed -E 's/.*"v?([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        log_error "Failed to fetch latest version"
        exit 1
    fi
    echo "$version"
}

# Validate version format
validate_version() {
    local version=$1
    if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        log_error "Invalid version format: $version (expected: X.Y.Z)"
        exit 1
    fi
}

# Download and extract Bootstrap Icons
download_icons() {
    local version=$1
    local download_url="https://github.com/twbs/icons/releases/download/v${version}/bootstrap-icons-${version}.zip"
    
    log_info "Downloading Bootstrap Icons v${version}..."
    
    if ! curl -sSL "$download_url" -o "$TEMP_DIR/bootstrap-icons.zip"; then
        log_error "Failed to download Bootstrap Icons v${version}"
        log_error "URL: $download_url"
        exit 1
    fi
    
    log_success "Downloaded Bootstrap Icons v${version}"
    
    log_info "Extracting archive..."
    if ! unzip -q "$TEMP_DIR/bootstrap-icons.zip" -d "$TEMP_DIR"; then
        log_error "Failed to extract archive"
        exit 1
    fi
    
    log_success "Extracted archive"
}

# Find the sprite file in the extracted archive
find_sprite() {
    local sprite_file=$(find "$TEMP_DIR" -name "bootstrap-icons.svg" -type f | grep -v node_modules | head -1)
    
    if [ -z "$sprite_file" ]; then
        log_error "Could not find bootstrap-icons.svg in the downloaded archive"
        log_info "Contents of $TEMP_DIR:"
        find "$TEMP_DIR" -type f | head -20
        exit 1
    fi
    
    echo "$sprite_file"
}

# Backup current sprite
backup_current() {
    if [ -f "$TARGET_PATH" ]; then
        local backup_path="${TARGET_PATH}.backup.$(date +%Y%m%d_%H%M%S)"
        log_info "Backing up current sprite to: $(basename $backup_path)"
        cp "$TARGET_PATH" "$backup_path"
        log_success "Backup created"
    else
        log_warning "No existing sprite found at $TARGET_PATH"
    fi
}

# Compare file sizes
compare_files() {
    local old_file=$1
    local new_file=$2
    
    if [ ! -f "$old_file" ]; then
        log_info "No previous file to compare"
        return
    fi
    
    local old_size=$(stat -f%z "$old_file" 2>/dev/null || stat -c%s "$old_file" 2>/dev/null)
    local new_size=$(stat -f%z "$new_file" 2>/dev/null || stat -c%s "$new_file" 2>/dev/null)
    local old_icons=$(grep -o '<symbol' "$old_file" | wc -l | tr -d ' ')
    local new_icons=$(grep -o '<symbol' "$new_file" | wc -l | tr -d ' ')
    
    log_info "File comparison:"
    echo "  Old: $(numfmt --to=iec-i --suffix=B $old_size 2>/dev/null || echo "${old_size} bytes") ($old_icons icons)"
    echo "  New: $(numfmt --to=iec-i --suffix=B $new_size 2>/dev/null || echo "${new_size} bytes") ($new_icons icons)"
    
    if [ "$new_icons" -gt "$old_icons" ]; then
        log_success "Added $(($new_icons - $old_icons)) new icons"
    elif [ "$new_icons" -lt "$old_icons" ]; then
        log_warning "Removed $(($old_icons - $new_icons)) icons"
    else
        log_info "Icon count unchanged"
    fi
}

# Show help
show_help() {
    cat << EOF
Bootstrap Icons Updater for Tailrelay Web UI

Usage:
  ./update-bootstrap-icons.sh [version]

Arguments:
  version    Optional version number (e.g., 1.11.3)
             If not specified, the latest release will be fetched

Examples:
  ./update-bootstrap-icons.sh        # Get latest version
  ./update-bootstrap-icons.sh 1.11.3 # Get specific version

This script will:
  1. Download Bootstrap Icons from GitHub releases
  2. Backup the current sprite file
  3. Extract and install the new sprite
  4. Compare file sizes and icon counts
  5. Provide next steps for testing and committing

EOF
    exit 0
}

# Main function
main() {
    local version=${1:-}
    
    # Show help if requested
    if [ "$version" = "-h" ] || [ "$version" = "--help" ]; then
        show_help
    fi
    
    # Header
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  Bootstrap Icons Updater for Tailrelay Web UI${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo
    
    # Check dependencies
    for cmd in curl unzip grep sed find; do
        if ! command -v $cmd &> /dev/null; then
            log_error "Required command not found: $cmd"
            exit 1
        fi
    done
    
    # Determine version
    if [ -z "$version" ]; then
        log_info "Fetching latest Bootstrap Icons version..."
        version=$(get_latest_version)
        log_success "Latest version: v${version}"
    else
        # Remove 'v' prefix if present
        version=${version#v}
        validate_version "$version"
        log_info "Using specified version: v${version}"
    fi
    
    # Download and extract
    download_icons "$version"
    
    # Find sprite file
    sprite_file=$(find_sprite)
    log_success "Found sprite: $(basename $(dirname $sprite_file))/$(basename $sprite_file)"
    
    # Backup current file
    backup_current
    
    # Compare files
    compare_files "$TARGET_PATH" "$sprite_file"
    
    # Create target directory if it doesn't exist
    mkdir -p "$(dirname $TARGET_PATH)"
    
    # Copy new sprite
    log_info "Installing new sprite..."
    cp "$sprite_file" "$TARGET_PATH"
    log_success "Sprite installed at: $TARGET_PATH"
    
    # Verify installation
    if [ -f "$TARGET_PATH" ]; then
        local icon_count=$(grep -o '<symbol' "$TARGET_PATH" | wc -l | tr -d ' ')
        log_success "Installation verified: $icon_count icons available"
    else
        log_error "Installation verification failed"
        exit 1
    fi
    
    # Next steps
    echo
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    log_success "Bootstrap Icons updated successfully!"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo
    log_info "Next steps:"
    echo "  1. Review changes: git diff $TARGET_PATH"
    echo "  2. Test the Web UI: make dev-build && make dev-docker-build"
    echo "  3. Commit changes: git add $TARGET_PATH && git commit -m 'Update Bootstrap Icons to v${version}'"
    echo
    log_info "View available icons at:"
    echo "  https://icons.getbootstrap.com/"
    echo
}

# Run main function
main "$@"
