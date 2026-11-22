#!/bin/sh
# Builds Loom.app (macOS) into dist, or into the folder given as the argument.
# Run from anywhere: sh packaging/macos/build-app.sh [folder]
set -e
cd "$(dirname "$0")/../.."
version=$(sed -n 's/^const version = "\(.*\)"/\1/p' internal/editor/main.go)
out=${1:-dist}
app=$out/Loom.app
rm -rf "$app" "$out/Loom.iconset"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" "$out/Loom.iconset"
go build -tags desktop -ldflags "-s -w" -o "$app/Contents/MacOS/loom" ./cmd/loom-desktop
sed "s/@VERSION@/$version/g" packaging/macos/Info.plist > "$app/Contents/Info.plist"
go run packaging/macos/icon.go "$out/icon.png"
for s in 16 32 128 256 512; do
	sips -z $s $s "$out/icon.png" --out "$out/Loom.iconset/icon_${s}x${s}.png" >/dev/null
	sips -z $((s * 2)) $((s * 2)) "$out/icon.png" --out "$out/Loom.iconset/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$out/Loom.iconset" -o "$app/Contents/Resources/Loom.icns"
codesign --force --deep --sign - "$app" >/dev/null 2>&1 || true
echo "Built $app (v$version)"
