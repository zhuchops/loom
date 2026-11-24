#!/bin/sh
# Builds Loom.app (macOS) into dist, or into the folder given as the argument.
# Run from anywhere: sh packaging/macos/build-app.sh [folder]
# LOOM_UNIVERSAL=1 builds one app for both Apple silicon and Intel Macs.
set -e
cd "$(dirname "$0")/../.."
version=$(sed -n 's/^const version = "\(.*\)"/\1/p' internal/editor/main.go)
out=${1:-dist}
app=$out/Loom.app
rm -rf "$app" "$out/Loom.iconset"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" "$out/Loom.iconset"
if [ "$LOOM_UNIVERSAL" = 1 ]; then
	# One app for Apple silicon and Intel: both halves, joined with lipo.
	CGO_ENABLED=1 GOARCH=arm64 CC="clang -arch arm64" go build -tags desktop,production -ldflags "-s -w" -o "$out/loom-arm64" ./cmd/loom-desktop
	CGO_ENABLED=1 GOARCH=amd64 CC="clang -arch x86_64" go build -tags desktop,production -ldflags "-s -w" -o "$out/loom-amd64" ./cmd/loom-desktop
	lipo -create "$out/loom-arm64" "$out/loom-amd64" -output "$app/Contents/MacOS/loom"
	rm -f "$out/loom-arm64" "$out/loom-amd64"
else
	go build -tags desktop,production -ldflags "-s -w" -o "$app/Contents/MacOS/loom" ./cmd/loom-desktop
fi
sed "s/@VERSION@/$version/g" packaging/macos/Info.plist > "$app/Contents/Info.plist"
go run packaging/macos/icon.go "$out/Loom.iconset"
iconutil -c icns "$out/Loom.iconset" -o "$app/Contents/Resources/Loom.icns"
codesign --force --deep --sign - "$app" >/dev/null 2>&1 || true
echo "Built $app (v$version)"
