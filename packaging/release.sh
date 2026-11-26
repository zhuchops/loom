#!/bin/sh
# Builds the files of a GitHub release into dist/release. The release
# workflow runs it on each kind of machine; it works the same by hand.
#
#   sh packaging/release.sh terminal   loom for macOS and Linux, arm64 and amd64
#   sh packaging/release.sh macos      Loom.app for Apple silicon and Intel (on a Mac)
#   sh packaging/release.sh linux      loom-desktop for this Linux machine
#
# Asset names carry no version, so .../releases/latest/download/<name> always
# points at the newest one; install.sh relies on that.
set -e
cd "$(dirname "$0")/.."
out=dist/release
mkdir -p "$out"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

case "$1" in
terminal)
	for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
		os=${target%/*}
		arch=${target#*/}
		mkdir -p "$work/$os-$arch"
		CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "$work/$os-$arch/loom" .
		tar -czf "$out/loom_${os}_${arch}.tar.gz" -C "$work/$os-$arch" loom
	done
	;;
macos)
	LOOM_UNIVERSAL=1 sh packaging/macos/build-app.sh "$work"
	ditto -c -k --norsrc --keepParent "$work/Loom.app" "$out/Loom_darwin_universal.zip"
	;;
linux)
	case $(uname -m) in
	x86_64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) echo "unsupported machine: $(uname -m)" >&2; exit 1 ;;
	esac
	go build -trimpath -tags desktop,production,gtk3 -ldflags "-s -w" -o "$work/loom-desktop" ./cmd/loom-desktop
	# The icon for the desktop entry that install.sh writes.
	go run packaging/linux/icon.go "$work/loom.png" 256
	tar -czf "$out/loom-desktop_linux_${arch}.tar.gz" -C "$work" loom-desktop loom.png
	;;
*)
	echo "usage: sh packaging/release.sh terminal|macos|linux" >&2
	exit 2
	;;
esac
ls -l "$out"
