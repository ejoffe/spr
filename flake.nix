{
  description = "spr - Stacked Pull Requests on GitHub";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        version = if (self ? rev) then self.shortRev else "dev";
        commit = if (self ? rev) then self.rev else "dirty";
        date = if (self ? lastModifiedDate) then self.lastModifiedDate else "unknown";
      in
      {
        packages = {
          spr = pkgs.buildGoModule {
            pname = "spr";
            inherit version;

            src = self;

            vendorHash = "sha256-byl+MF0vlfa4V/3uPrv5Qlcvh5jIozEyUkKSSwlRWhs=";

            subPackages = [
              "cmd/spr"
              "cmd/amend"
              "cmd/reword"
            ];

            ldflags = [
              "-s" "-w"
              "-X main.version=${version}"
              "-X main.commit=${commit}"
              "-X main.date=${date}"
              "-X main.builtBy=nix"
            ];

            postInstall = ''
              mv $out/bin/spr $out/bin/git-spr
              mv $out/bin/amend $out/bin/git-amend
              mv $out/bin/reword $out/bin/spr_reword_helper
            '';

            meta = with pkgs.lib; {
              description = "Stacked Pull Requests on GitHub";
              homepage = "https://github.com/ejoffe/spr";
              license = licenses.mit;
              mainProgram = "git-spr";
            };
          };

          default = self.packages.${system}.spr;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            goreleaser
            git
          ];
        };
      }
    );
}
