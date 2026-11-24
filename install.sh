#!/bin/sh
# Installs Loom from its GitHub releases.
#
#   curl -fsSL https://raw.githubusercontent.com/zhuchops/loom/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/zhuchops/loom/main/install.sh | sh -s -- --desktop
#
# Options:
#   --desktop        also install the desktop app (Loom.app on macOS, loom-desktop on Linux)
#   --version vX.Y.Z install that release instead of the newest
#   --dir PATH       where the commands go (default: ~/.local/bin)
#
# The same can be set with LOOM_DESKTOP=1, LOOM_VERSION and LOOM_INSTALL_DIR;
# LOOM_APPS_DIR puts Loom.app somewhere other than /Applications.
set -eu

repo=zhuchops/loom
base=${LOOM_BASE_URL:-https://github.com/$repo/releases}
version=${LOOM_VERSION:-}
dir=${LOOM_INSTALL_DIR:-$HOME/.local/bin}
desktop=${LOOM_DESKTOP:-0}

while [ $# -gt 0 ]; do
	case "$1" in
	--desktop) desktop=1 ;;
	--version) version=$2; shift ;;
	--dir) dir=$2; shift ;;
	-h | --help) sed -n '2,13p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//'; exit 0 ;;
	*) echo "loom install: unknown option $1" >&2; exit 2 ;;
	esac
	shift
done

say() { printf '%s\n' "$*"; }
fail() { printf 'loom install: %s\n' "$*" >&2; exit 1; }

case $(uname -s) in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) fail "no release for $(uname -s); build from source with go install github.com/$repo@latest" ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) fail "no release for $(uname -m)" ;;
esac

if [ -n "$version" ]; then
	url=$base/download/$version
else
	url=$base/latest/download
fi

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q "$1" -O "$2"; }
else
	fail "curl or wget is needed"
fi
if command -v sha256sum >/dev/null 2>&1; then
	sha() { sha256sum "$1" | cut -d' ' -f1; }
else
	sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fetch "$url/checksums.txt" "$tmp/checksums.txt" || fail "could not download $url/checksums.txt"

# get downloads a release file and checks it against checksums.txt.
get() {
	fetch "$url/$1" "$tmp/$1" || fail "could not download $url/$1"
	want=$(grep " $1\$" "$tmp/checksums.txt" | cut -d' ' -f1)
	[ -n "$want" ] || fail "$1 is not in checksums.txt"
	[ "$(sha "$tmp/$1")" = "$want" ] || fail "$1 does not match its checksum"
}

mkdir -p "$dir"
get "loom_${os}_${arch}.tar.gz"
tar -xzf "$tmp/loom_${os}_${arch}.tar.gz" -C "$tmp"
install -m 755 "$tmp/loom" "$dir/loom"
say "Installed $dir/loom ($("$dir/loom" -version))"

# Without the window version, loom-desktop is loom under another name: in an
# SSH session started with "loom ssh" it opens files in the Loom app on the
# computer the session came from.
if [ "$desktop" != 1 ] && [ ! -e "$dir/loom-desktop" ]; then
	ln -s loom "$dir/loom-desktop"
fi

if [ "$desktop" = 1 ]; then
	if [ "$os" = darwin ]; then
		get Loom_darwin_universal.zip
		apps=${LOOM_APPS_DIR:-/Applications}
		[ -w "$apps" ] || apps=$HOME/Applications
		mkdir -p "$apps"
		rm -rf "$apps/Loom.app"
		ditto -x -k "$tmp/Loom_darwin_universal.zip" "$apps"
		# The app is signed ad hoc, not notarized; files fetched by curl carry
		# no quarantine flag, but clear it in case.
		xattr -dr com.apple.quarantine "$apps/Loom.app" 2>/dev/null || true
		ln -sf "$apps/Loom.app/Contents/MacOS/loom" "$dir/loom-desktop"
		say "Installed $apps/Loom.app and the command $dir/loom-desktop"
	else
		get "loom-desktop_linux_${arch}.tar.gz"
		tar -xzf "$tmp/loom-desktop_linux_${arch}.tar.gz" -C "$tmp"
		install -m 755 "$tmp/loom-desktop" "$dir/loom-desktop"
		say "Installed $dir/loom-desktop"
		if ! (ldconfig -p 2>/dev/null || true) | grep -q 'libwebkit2gtk-4.1'; then
			say "loom-desktop needs WebKitGTK 4.1 and GTK 3, which this system does not seem to have:"
			say "  Debian, Ubuntu: sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0"
			say "  Fedora:         sudo dnf install webkit2gtk4.1 gtk3"
			say "  Arch:           sudo pacman -S webkit2gtk-4.1 gtk3"
		fi
	fi
fi

case ":$PATH:" in
*":$dir:"*) ;;
*) say "Add $dir to your PATH, for example: echo 'export PATH=\"$dir:\$PATH\"' >> ~/.zshrc" ;;
esac
