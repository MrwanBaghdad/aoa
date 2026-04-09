{
  description = "aoa — run AI coding agents in isolated macOS VMs";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "aarch64-darwin" "x86_64-darwin" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {

        # ── installable package ──────────────────────────────────────────────
        packages.default = pkgs.buildGoModule {
          pname = "aoa";
          version = "0.1.0";

          src = ./.;

          vendorHash = null; # update with `nix build` output on first run

          ldflags = [ "-s" "-w" ];

          meta = {
            description = "Run AI coding agents in isolated macOS VMs";
            homepage = "https://github.com/marwan/aoa";
            license = pkgs.lib.licenses.mit;
            mainProgram = "aoa";
            platforms = pkgs.lib.platforms.darwin;
          };
        };

        # ── dev shell (replaces devbox) ──────────────────────────────────────
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            tmux
            # Python test harness
            (python3.withPackages (ps: with ps; [ pytest pexpect pyte ]))
          ];

          shellHook = ''
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
            if ! command -v container &>/dev/null; then
              echo "error: apple/container is not installed — aoa requires it to run VMs."
              echo "       Install from: https://github.com/apple/container/releases"
              exit 1
            fi
            echo "aoa dev shell — go $(go version | awk '{print $3}')"
          '';
        };

      });
}
