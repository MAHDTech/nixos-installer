#!/usr/bin/env bash

# Name: yolo.sh
# Description: A one-shot script to run the nixos-installer.

clear
set -euo pipefail

#########################
# Variables
#########################

GITHUB_REPO="MAHDTech/nixos-installer"
GITHUB_TAG="latest"

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
	local ARCH

	ARCH="$(uname -m)"

	case "$ARCH" in

	x86_64)
		msg "INFO" "Detected x86_64 architecture"
		ARCH="amd64"
		;;

	aarch64)
		msg "INFO" "Detected aarch64 architecture"
		ARCH="arm64"
		;;

	*)
		msg "ERROR" "Unsupported architecture: $ARCH"
		exit 1
		;;
	esac

	return 0
}

function detect_os() {
	local OS

	OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

	case "$OS" in

	linux)
		msg "INFO" "Detected Linux OS"
		;;

	*)
		msg "ERROR" "Unsupported OS: $OS"
		exit 1
		;;
	esac

	return 0
}

# shellcheck disable=SC2317
function cleanup() {
	rm -rf "${TMP_DIR}" || {
		msg "ERROR" "Failed to clean up temporary directory"
		return 1
	}
	return 0
}

#########################
# Main
#########################

# Setup a trap to cleanup on exit.
trap cleanup EXIT SIGINT SIGTERM || {
	msg "ERROR" "Failed to setup trap for cleanup"
	exit 1
}

# Detect architecture
detect_architecture || {
	msg "ERROR" "Failed to detect architecture"
	exit 1
}

# Detect OS
detect_os || {
	msg "ERROR" "Failed to detect OS"
	exit 1
}

# Define the binary name and URL
BINARY="nixos-installer-${OS}-${ARCH}"
URL="https://github.com/${GITHUB_REPO}/releases/download/${GITHUB_TAG}/${BINARY}"

# Download binary
curl -fsSL "$URL" -o "${TMP_DIR}/${BINARY}" || {
	msg "ERROR" "Failed to download binary"
	exit 1
}

# Make executable
chmod +x "${TMP_DIR}/${BINARY}" || {
	msg "ERROR" "Failed to make binary executable"
	exit 1
}

# Run with sudo and pass all arguments
sudo "${TMP_DIR}/${BINARY}" "$@" || {
	msg "ERROR" "Failed to run binary"
	exit 1
}

exit 0
