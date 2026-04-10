# typed: false
# frozen_string_literal: true

class Aoa < Formula
  desc "Run AI coding agents in isolated macOS VMs (Agent on Apple)"
  homepage "https://github.com/MrwanBaghdad/aoa"
  license "MIT"
  head "https://github.com/MrwanBaghdad/aoa.git", branch: "main"

  # Stable url/version/sha256 are added here by `make formula VERSION=x.y.z`
  # when a release is tagged.

  bottle do
    # bottles are built by CI and uploaded alongside releases
  end

  depends_on :macos
  depends_on "go" => :build

  def install
    system "go", "build",
           "-ldflags", "-s -w",
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
