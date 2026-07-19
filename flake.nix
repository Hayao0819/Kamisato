{
  description = "Kamisato — ayato (repo server), miko (build server), ayaka (CLI), kayo (AUR overlay), lumine (web BFF)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;

      # nixpkgs + this flake's overlay; packages/nixosModules derive from it.
      pkgsFor = forAllSystems (
        system:
        import nixpkgs {
          inherit system;
          overlays = [ self.overlays.default ];
        }
      );
    in
    {
      overlays.default =
        final: _prev:
        let
          inherit (final) buildGoModule;
          version = "0.0.2-${self.shortRev or self.dirtyShortRev or "dirty"}";

          src = self; # git-aware: excludes node_modules / built embed/out

          # Shared by every binary; refresh via `nix build .#ayato` on go.mod changes.
          vendorHash = "sha256-gHpTtJ7R4nuALqme480ZYJKcrkxoF+fytO5bYW5OPEQ=";

          mkGo =
            args:
            buildGoModule (
              {
                inherit version src vendorHash;
                ldflags = [
                  "-s"
                  "-w"
                ];
                # CGO stays on (ayato uses mattn/go-sqlite3); tests need network/Landlock.
                doCheck = false;
              }
              // args
            );

          # Next.js console embedded by lumine/kamisato; built separately, copied in below.
          lumine-web = final.stdenv.mkDerivation (finalAttrs: {
            pname = "lumine-web";
            inherit version;
            src = self + "/lumine/web";

            nativeBuildInputs = [
              final.nodejs
              final.pnpm_10
              final.pnpmConfigHook
            ];

            # Refresh via `nix build .#lumine` on pnpm-lock.yaml changes.
            pnpmDeps = final.fetchPnpmDeps {
              inherit (finalAttrs) pname version src;
              pnpm = final.pnpm_10;
              fetcherVersion = 4;
              hash = "sha256-LXlyYsKrT9k+cPGDcN4RDYtyvTSazFlHA9V+/bq0Rr8=";
            };

            env = {
              NEXT_TELEMETRY_DISABLED = "1";
              NODE_ENV = "production";
            };

            buildPhase = ''
              runHook preBuild
              pnpm run build
              runHook postBuild
            '';

            # next.config exports the static site to ../embed/out.
            installPhase = ''
              runHook preInstall
              cp -r ../embed/out "$out"
              runHook postInstall
            '';
          });

          # lumine & kamisato import lumine/embed (//go:embed out/**): supply the frontend.
          embedFrontend = ''
            rm -rf lumine/embed/out
            cp -r ${lumine-web} lumine/embed/out
          '';
        in
        {
          inherit lumine-web;

          ayato = mkGo {
            pname = "ayato";
            subPackages = [ "ayato" ];
            meta = {
              description = "Kamisato package-repository server";
              mainProgram = "ayato";
            };
          };
          ayaka = mkGo {
            pname = "ayaka";
            subPackages = [ "ayaka" ];
            meta = {
              description = "Kamisato CLI";
              mainProgram = "ayaka";
            };
          };
          miko = mkGo {
            pname = "miko";
            subPackages = [ "miko" ];
            meta = {
              description = "Kamisato build server";
              mainProgram = "miko";
            };
          };
          kayo = mkGo {
            pname = "kayo";
            subPackages = [ "kayo" ];
            meta = {
              description = "Kamisato AUR overlay router";
              mainProgram = "kayo";
            };
          };

          lumine = mkGo {
            pname = "lumine";
            subPackages = [ "lumine" ];
            postPatch = embedFrontend;
            meta = {
              description = "Kamisato web BFF (lumine)";
              mainProgram = "lumine";
            };
          };
          kamisato = mkGo {
            pname = "kamisato";
            subPackages = [ "." ];
            postPatch = embedFrontend;
            meta = {
              description = "Unified Kamisato binary (all subcommands)";
              mainProgram = "kamisato";
            };
          };
        };

      packages = forAllSystems (
        system:
        let
          p = pkgsFor.${system};
        in
        {
          inherit (p)
            ayato
            ayaka
            miko
            kayo
            lumine
            kamisato
            ;
          default = p.kamisato;
        }
      );

      devShells = forAllSystems (
        system:
        let
          p = pkgsFor.${system};
        in
        {
          default = p.mkShell {
            packages = [
              p.go
              p.gopls
              p.gotools
              p.go-tools # staticcheck
              p.golangci-lint
              p.delve
              p.nodejs
              p.pnpm_10
              p.biome
              p.nixfmt
            ];
          };
        }
      );

      nixosModules.default = import ./nix/module.nix;

      formatter = forAllSystems (system: pkgsFor.${system}.nixfmt);
    };
}
