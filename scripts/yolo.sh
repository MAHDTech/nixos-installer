#!/usr/bin/env bash

# Name: yolo.sh
# Description: A one-shot script to run the nixos-installer.

clear
set -euo pipefail

#########################
# Variables
#########################

GITHUB_PROJECT="nixos-installer"
GITHUB_REPO="MAHDTech/nixos-installer"
GITHUB_TAG="latest"

CHECKSUM_FILE="checksums.txt"

declare DETECTED_ARCH
declare DETECTED_OS

TMP_DIR=$(mktemp -d)

#########################
# Functions
#########################

function msg() {
	local LEVEL="${1:-"INFO"}"
	local MESSAGE="${2:-"No message provided."}"

	case "${LEVEL^^}" in
	"DEBUG")
		echo -e "\033[32m[DEBUG] ${MESSAGE}\033[0m"
		;;
	"INFO")
		echo -e "\033[34m[INFO] ${MESSAGE}\033[0m"
		;;
	"WARN")
		echo -e "\033[33m[WARN] ${MESSAGE}\033[0m"
		;;
	"ERROR")
		echo -e "\033[31m[ERROR] ${MESSAGE}\033[0m"
		;;
	*) ;;
	esac

	return 0
}

function detect_architecture() {
	DETECTED_ARCH="$(uname -m)"

	case "$DETECTED_ARCH" in

	x86_64)
		msg "INFO" "✅ Detected x86_64 architecture"
		ARCH="amd64"
		;;

	aarch64)
		msg "INFO" "✅ Detected aarch64 architecture"
		ARCH="arm64"
		;;

	*)
		msg "ERROR" "❌ Unsupported architecture: $ARCH"
		exit 1
		;;
	esac

	return 0
}

function detect_os() {
	DETECTED_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

	case "$DETECTED_OS" in

	linux)
		msg "INFO" "✅ Detected Linux OS"
		;;

	darwin)
		msg "INFO" "✅ Detected macOS OS"
		;;

	*)
		msg "ERROR" "❌ Unsupported OS: $DETECTED_OS"
		exit 1
		;;
	esac

	return 0
}

# shellcheck disable=SC2317,SC2329
function cleanup() {
	msg "INFO" "🧹 Cleaning up temporary directory"
	rm -rf "${TMP_DIR}" || {
		msg "ERROR" "❌ Failed to clean up temporary directory"
		return 1
	}
	return 0
}

#########################
# Main
#########################

# Setup a trap to cleanup on exit.
trap cleanup EXIT SIGINT SIGTERM || {
	msg "ERROR" "❌ Failed to setup trap for cleanup"
	exit 1
}

# Detect architecture
detect_architecture || {
	msg "ERROR" "❌ Failed to detect architecture"
	exit 1
}

# Detect OS
detect_os || {
	msg "ERROR" "❌ Failed to detect OS"
	exit 1
}

# Define the binary name and URL
BINARY="${GITHUB_PROJECT}-${DETECTED_OS}-${ARCH}"

# Define the correct URLs based on the tag.
case "$GITHUB_TAG" in

latest)
	msg "INFO" "🚀 Downloading latest release"
	RELEASE_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${BINARY}"
	CHECKSUM_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${CHECKSUM_FILE}"
	;;
*)
	msg "INFO" "🚀 Downloading release ${GITHUB_TAG}"
	RELEASE_URL="https://github.com/${GITHUB_REPO}/releases/download/${GITHUB_TAG}/${BINARY}"
	CHECKSUM_URL="https://github.com/${GITHUB_REPO}/releases/download/${GITHUB_TAG}/${CHECKSUM_FILE}"
	;;
esac

# Download checksums file
msg "INFO" "🔑 Downloading checksums file from ${CHECKSUM_URL}"
curl -fsSL "$CHECKSUM_URL" -o "${TMP_DIR}/${CHECKSUM_FILE}" || {
	msg "ERROR" "❌ Failed to download checksums file"
	exit 1
}

# Download binary
msg "INFO" "🚀 Downloading binary from ${RELEASE_URL}"
curl -fsSL "$RELEASE_URL" -o "${TMP_DIR}/${BINARY}" || {
	msg "ERROR" "❌ Failed to download binary"
	exit 1
}

# Calculate the checksum of the downloaded binary
BINARY_CHECKSUM="$(sha256sum "${TMP_DIR}/${BINARY}" | awk '{print $1}')"

# Extract the expected checksum from the checksums.txt file
EXPECTED_CHECKSUM="$(awk -v bin="${BINARY}" '$2 == bin {print $1}' "${TMP_DIR}/${CHECKSUM_FILE}")"

if [[ -z ${EXPECTED_CHECKSUM} ]]; then
	msg "ERROR" "❌ Could not find expected checksum for ${BINARY} in checksums.txt"
	exit 1
fi

if [[ ${BINARY_CHECKSUM} != "${EXPECTED_CHECKSUM}" ]]; then
	msg "ERROR" "🛑 Checksum mismatch! Expected: ${EXPECTED_CHECKSUM}, Got: ${BINARY_CHECKSUM}"
	exit 1
else
	msg "INFO" "🔑 Checksum verified! (${BINARY_CHECKSUM})"
fi

# Make executable
chmod +x "${TMP_DIR}/${BINARY}" || {
	msg "ERROR" "❌ Failed to make binary executable"
	exit 1
}

# Run with sudo and pass all arguments
sudo "${TMP_DIR}/${BINARY}" "$@" || {
	msg "ERROR" "❌ Failed to run binary"
	exit 1
}

exit 0
