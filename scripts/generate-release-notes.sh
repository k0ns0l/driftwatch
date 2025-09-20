#!/bin/bash

# DriftWatch Release Notes Generator
# This script generates release notes from the template and git history

set -euo pipefail

# Configuration
TEMPLATE_FILE=".github/RELEASE_TEMPLATE.md"
OUTPUT_FILE="RELEASE_NOTES.md"
REPO_URL="https://github.com/k0ns0l/driftwatch"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to get the latest tag
get_latest_tag() {
    git describe --tags --abbrev=0 2>/dev/null || echo "v1.0.0"
}

# Function to get the previous tag
get_previous_tag() {
    local current_tag="$1"
    git describe --tags --abbrev=0 "${current_tag}^" 2>/dev/null || echo "v0.0.0"
}

# Function to get commit count between tags
get_commit_count() {
    local from_tag="$1"
    local to_tag="$2"
    git rev-list --count "${from_tag}..${to_tag}" 2>/dev/null || echo "0"
}

# Function to get contributors between tags
get_contributors() {
    local from_tag="$1"
    local to_tag="$2"
    git log --format='%aN <%aE>' "${from_tag}..${to_tag}" | sort -u | while read -r contributor; do
        echo "- ${contributor}"
    done
}

# Function to get file statistics
get_file_stats() {
    local from_tag="$1"
    local to_tag="$2"
    git diff --stat "${from_tag}..${to_tag}" | tail -n 1 | sed 's/^ *//'
}

# Function to extract changelog content for version
extract_changelog_content() {
    local version="$1"
    if [[ -f "CHANGELOG.md" ]]; then
        awk "/^## \[${version}\]/{flag=1; next} /^## \[/{flag=0} flag" CHANGELOG.md
    else
        echo "No changelog content available."
    fi
}

# Function to generate release notes
generate_release_notes() {
    local version="$1"
    local tag="v${version}"
    local previous_tag
    local previous_version
    local commit_count
    local contributors
    local file_stats
    local changelog_content
    local release_date

    log_info "Generating release notes for version ${version}"

    # Get previous tag and version
    previous_tag=$(get_previous_tag "${tag}")
    previous_version="${previous_tag#v}"
    
    # Get statistics
    commit_count=$(get_commit_count "${previous_tag}" "${tag}")
    contributors=$(get_contributors "${previous_tag}" "${tag}")
    file_stats=$(get_file_stats "${previous_tag}" "${tag}")
    changelog_content=$(extract_changelog_content "${version}")
    release_date=$(date +"%Y-%m-%d")

    # Check if template exists
    if [[ ! -f "${TEMPLATE_FILE}" ]]; then
        log_error "Release template not found: ${TEMPLATE_FILE}"
        exit 1
    fi

    # Copy template and replace placeholders
    cp "${TEMPLATE_FILE}" "${OUTPUT_FILE}"

    # Replace placeholders
    sed -i "s/{VERSION}/${version}/g" "${OUTPUT_FILE}"
    sed -i "s/{TAG}/${tag}/g" "${OUTPUT_FILE}"
    sed -i "s/{PREVIOUS_VERSION}/${previous_version}/g" "${OUTPUT_FILE}"
    sed -i "s/{PREVIOUS_TAG}/${previous_tag}/g" "${OUTPUT_FILE}"
    sed -i "s/{DATE}/${release_date}/g" "${OUTPUT_FILE}"
    sed -i "s/{COMMIT_COUNT}/${commit_count}/g" "${OUTPUT_FILE}"

    # Replace multiline content
    if [[ -n "${contributors}" ]]; then
        # Create temporary file with contributors
        echo "${contributors}" > /tmp/contributors.txt
        # Replace the placeholder with file content
        sed -i "/{CONTRIBUTORS}/r /tmp/contributors.txt" "${OUTPUT_FILE}"
        sed -i "/{CONTRIBUTORS}/d" "${OUTPUT_FILE}"
        rm -f /tmp/contributors.txt
    else
        sed -i "s/{CONTRIBUTORS}/No contributors found./g" "${OUTPUT_FILE}"
    fi

    if [[ -n "${changelog_content}" ]]; then
        # Create temporary file with changelog content
        echo "${changelog_content}" > /tmp/changelog.txt
        # Replace the placeholder with file content
        sed -i "/{CHANGELOG_CONTENT}/r /tmp/changelog.txt" "${OUTPUT_FILE}"
        sed -i "/{CHANGELOG_CONTENT}/d" "${OUTPUT_FILE}"
        rm -f /tmp/changelog.txt
    else
        sed -i "s/{CHANGELOG_CONTENT}/No changelog content available./g" "${OUTPUT_FILE}"
    fi

    # Parse file statistics
    if [[ -n "${file_stats}" ]]; then
        local files_changed=$(echo "${file_stats}" | grep -o '[0-9]\+ files\? changed' | grep -o '[0-9]\+' || echo "0")
        local lines_added=$(echo "${file_stats}" | grep -o '[0-9]\+ insertions\?' | grep -o '[0-9]\+' || echo "0")
        local lines_removed=$(echo "${file_stats}" | grep -o '[0-9]\+ deletions\?' | grep -o '[0-9]\+' || echo "0")
        
        sed -i "s/{FILES_CHANGED}/${files_changed}/g" "${OUTPUT_FILE}"
        sed -i "s/{LINES_ADDED}/${lines_added}/g" "${OUTPUT_FILE}"
        sed -i "s/{LINES_REMOVED}/${lines_removed}/g" "${OUTPUT_FILE}"
    else
        sed -i "s/{FILES_CHANGED}/0/g" "${OUTPUT_FILE}"
        sed -i "s/{LINES_ADDED}/0/g" "${OUTPUT_FILE}"
        sed -i "s/{LINES_REMOVED}/0/g" "${OUTPUT_FILE}"
    fi

    log_success "Release notes generated: ${OUTPUT_FILE}"
}

# Function to validate release notes
validate_release_notes() {
    local file="$1"
    
    log_info "Validating release notes..."
    
    # Check for remaining placeholders
    local placeholders
    placeholders=$(grep -o '{[A-Z_]*}' "${file}" || true)
    
    if [[ -n "${placeholders}" ]]; then
        log_warning "Found unreplaced placeholders:"
        echo "${placeholders}"
    else
        log_success "No unreplaced placeholders found"
    fi
    
    # Check file size
    local file_size
    file_size=$(wc -c < "${file}")
    
    if [[ "${file_size}" -lt 1000 ]]; then
        log_warning "Release notes file seems small (${file_size} bytes)"
    else
        log_success "Release notes file size looks good (${file_size} bytes)"
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS] [VERSION]"
    echo ""
    echo "Generate release notes for DriftWatch"
    echo ""
    echo "OPTIONS:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --validate Validate existing release notes"
    echo "  -o, --output   Output file (default: release-notes.md)"
    echo ""
    echo "ARGUMENTS:"
    echo "  VERSION        Version to generate notes for (default: latest tag)"
    echo ""
    echo "Examples:"
    echo "  $0                    # Generate notes for latest tag"
    echo "  $0 1.2.0             # Generate notes for version 1.2.0"
    echo "  $0 --validate         # Validate existing release notes"
    echo "  $0 -o notes.md 1.2.0  # Generate notes to custom file"
}

# Main function
main() {
    local version=""
    local validate_only=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--validate)
                validate_only=true
                shift
                ;;
            -o|--output)
                OUTPUT_FILE="$2"
                shift 2
                ;;
            -*)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
            *)
                version="$1"
                shift
                ;;
        esac
    done
    
    # Validate only mode
    if [[ "${validate_only}" == true ]]; then
        if [[ -f "${OUTPUT_FILE}" ]]; then
            validate_release_notes "${OUTPUT_FILE}"
        else
            log_error "Release notes file not found: ${OUTPUT_FILE}"
            exit 1
        fi
        exit 0
    fi
    
    # Get version if not provided
    if [[ -z "${version}" ]]; then
        local latest_tag
        latest_tag=$(get_latest_tag)
        version="${latest_tag#v}"
        log_info "Using latest tag version: ${version}"
    fi
    
    # Validate version format
    if [[ ! "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
        log_error "Invalid version format: ${version}"
        log_error "Expected format: X.Y.Z or X.Y.Z-suffix"
        exit 1
    fi
    
    # Check if we're in a git repository
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        log_error "Not in a git repository"
        exit 1
    fi
    
    # Generate release notes
    generate_release_notes "${version}"
    
    # Validate generated notes
    validate_release_notes "${OUTPUT_FILE}"
    
    log_success "Release notes generation completed!"
    log_info "File: ${OUTPUT_FILE}"
    log_info "You can now review and edit the release notes before publishing."
}

# Run main function
main "$@"