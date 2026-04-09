# typed: false
# frozen_string_literal: true

class Aoa < Formula
  desc "Run AI coding agents in isolated macOS VMs (Agent on Apple)"
  homepage "https://github.com/marwan/aoa"
  url "https://github.com/marwan/aoa/archive/refs/tags/v#{version}.tar.gz"
  sha256 "" # filled in by `make formula` or goreleaser
  license "MIT"
  head "https://github.com/marwan/aoa.git", branch: "main"

  bottle do
    # bottles are built by CI and uploaded alongside the release
  end

  depends_on :macos
  depends_on "go" => :build

  def install
    system "go", "build",
           "-ldflags", "-s -w -X main.version=#{version}",
           "-o", bin/"aoa",
           "."
  end

  def caveats
    <<~EOS
      aoa requires apple/container to run VMs:
        https://github.com/apple/container

      To get started:
        aoa build          # build the agent container image
        aoa health         # verify all dependencies
        aoa shell <dir>    # run Claude Code in a sandboxed VM
    EOS
  end

  test do
    assert_match "aoa", shell_output("#{bin}/aoa --help")
    assert_match "aoa health check", shell_output("#{bin}/aoa health", 1)
  end
end
