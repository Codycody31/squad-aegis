{
  description = "Squad Aegis — a comprehensive control panel for Squad game server administration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      # Systems we build for. Kept explicit rather than pulling in flake-utils
      # so the flake has a single input and stays auditable.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;

      pkgsFor = forAllSystems (
        system:
        import nixpkgs {
          inherit system;
          overlays = [ self.overlays.default ];
        }
      );
    in
    {
      # Overlay so downstream flakes / configs can `pkgs.squad-aegis`.
      overlays.default = final: prev: {
        squad-aegis-web = final.callPackage ./nix/web.nix { };
        squad-aegis = final.callPackage ./nix/package.nix {
          inherit (self) sourceInfo;
          squad-aegis-web = final.squad-aegis-web;
        };
      };

      packages = forAllSystems (
        system:
        let
          pkgs = pkgsFor.${system};
        in
        {
          default = pkgs.squad-aegis;
          squad-aegis = pkgs.squad-aegis;
          squad-aegis-web = pkgs.squad-aegis-web;
        }
      );

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.squad-aegis}/bin/squad-aegis";
        };
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = pkgsFor.${system};
        in
        {
          default = pkgs.callPackage ./nix/devshell.nix { };
        }
      );

      # `nix build .#checks.<system>.*` / used by `nix flake check`.
      checks = forAllSystems (system: {
        inherit (self.packages.${system}) squad-aegis squad-aegis-web;
        devShell = self.devShells.${system}.default;
      });

      formatter = forAllSystems (system: pkgsFor.${system}.nixfmt);

      nixosModules.default = {
        imports = [ ./nix/module.nix ];
        # Make `pkgs.squad-aegis` resolvable wherever the module is imported.
        nixpkgs.overlays = [ self.overlays.default ];
      };
    };
}
