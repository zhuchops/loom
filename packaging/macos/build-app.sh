#!/bin/sh
# Builds dist/Loom.app (macOS). Run from anywhere: sh packaging/macos/build-app.sh
set -e
cd "$(dirname "$0")/../.."
version=$(sed -n 's/^const version = "\(.*\)"/\1/p' internal/editor/main.go)
app=dist/Loom.app
rm -rf "$app" dist/Loom.iconset
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" dist/Loom.iconset
go build -tags desktop -ldflags "-s -w" -o "$app/Contents/MacOS/loom" ./cmd/loom-desktop
sed "s/@VERSION@/$version/g" packaging/macos/Info.plist > "$app/Contents/Info.plist"
go run packaging/macos/icon.go dist/icon.png
for s in 16 32 128 256 512; do
	sips -z $s $s dist/icon.png --out "dist/Loom.iconset/icon_${s}x${s}.png" >/dev/null
	sips -z $((s * 2)) $((s * 2)) dist/icon.png --out "dist/Loom.iconset/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns dist/Loom.iconset -o "$app/Contents/Resources/Loom.icns"
codesign --force --deep --sign - "$app" >/dev/null 2>&1 || true
echo "Built $app (v$version)"
