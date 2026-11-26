# Homebrew formula for Loom. This repository is its own tap:
#
#   brew tap zhuchops/loom https://github.com/zhuchops/loom
#   brew install loom
#
# It builds the terminal editor from source. The window version is the cask
# loom-desktop on macOS (brew install --cask loom-desktop) and install.sh
# --desktop on Linux, where it needs the system's WebKitGTK.
class Loom < Formula
  desc "Vim-style code editor for the terminal and the desktop"
  homepage "https://github.com/zhuchops/loom"
  url "https://github.com/zhuchops/loom.git", tag: "v0.43.0"
  head "https://github.com/zhuchops/loom.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"loom"), "."
    # On a server, loom-desktop opens files in the Loom app on the computer
    # an SSH session came from (loom ssh); there it is loom by another name.
    bin.install_symlink "loom" => "loom-desktop" if OS.linux?
  end

  test do
    assert_match "loom #{version}", shell_output("#{bin}/loom -version")
  end
end
