# Homebrew formula for Loom. This repository is its own tap:
#
#   brew tap zhuchops/loom https://github.com/zhuchops/loom
#   brew install loom
#
# It builds from source. On macOS it also builds loom-desktop, the window
# version; on Linux that one needs WebKitGTK, so use install.sh --desktop.
class Loom < Formula
  desc "Vim-style code editor for the terminal and the desktop"
  homepage "https://github.com/zhuchops/loom"
  url "https://github.com/zhuchops/loom.git", tag: "v0.31.0"
  head "https://github.com/zhuchops/loom.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"loom"), "."
    if OS.mac?
      system "go", "build", "-tags", "desktop,production",
             *std_go_args(ldflags: "-s -w", output: bin/"loom-desktop"), "./cmd/loom-desktop"
    end
  end

  test do
    assert_match "loom #{version}", shell_output("#{bin}/loom -version")
  end
end
