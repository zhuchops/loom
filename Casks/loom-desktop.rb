# Homebrew cask for Loom.app, the window version of Loom, from the GitHub
# releases. This repository is its own tap:
#
#   brew tap zhuchops/loom https://github.com/zhuchops/loom
#   brew install --cask loom-desktop
#
# The release workflow updates version and sha256 for every release.
cask "loom-desktop" do
  version "0.37.0"
  sha256 "368638520b3bb0cd8ab658e9c0dda17233cbcec7aad3994d41440f0f74322f0b"

  url "https://github.com/zhuchops/loom/releases/download/v#{version}/Loom_darwin_universal.zip"
  name "Loom"
  desc "Vim-style code editor"
  homepage "https://github.com/zhuchops/loom"

  depends_on :macos

  app "Loom.app"
  binary "#{appdir}/Loom.app/Contents/MacOS/loom", target: "loom-desktop"

  # The app is signed ad hoc, not notarized, so macOS would refuse to open
  # what Homebrew downloaded and marked as quarantined.
  postflight_steps do
    run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{appdir}}/Loom.app"], must_succeed: false,
                          writable_paths: ["Loom.app"], writable_base: :appdir
  end

  zap trash: [
    "~/.config/loom",
    "~/.local/state/loom",
  ]
end
