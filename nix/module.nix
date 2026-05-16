{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.squad-aegis;

  # Settings are surfaced to the binary as environment variables (the same
  # ones documented in .env.example). Booleans/ints are stringified so the
  # option type can stay ergonomic.
  toEnvValue = v: if builtins.isBool v then lib.boolToString v else toString v;
  environment = lib.mapAttrs (_: toEnvValue) cfg.settings;
in
{
  options.services.squad-aegis = {
    enable = lib.mkEnableOption "Squad Aegis control panel";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.squad-aegis;
      defaultText = lib.literalExpression "pkgs.squad-aegis";
      description = "The squad-aegis package to run.";
    };

    settings = lib.mkOption {
      type =
        with lib.types;
        attrsOf (oneOf [
          str
          int
          bool
        ]);
      default = { };
      example = lib.literalExpression ''
        {
          APP_URL = "https://aegis.example.com";
          DB_HOST = "127.0.0.1";
          DB_NAME = "squad-aegis";
          DB_USER = "squad-aegis";
        }
      '';
      description = ''
        Configuration exposed as environment variables. See `.env.example`
        in the source tree for the full list. Secrets (passwords, S3 keys)
        belong in {option}`services.squad-aegis.environmentFile` instead.
      '';
    };

    environmentFile = lib.mkOption {
      type = with lib.types; nullOr path;
      default = null;
      example = "/run/secrets/squad-aegis.env";
      description = ''
        Path to an environment file (systemd `EnvironmentFile`) holding
        secrets such as `DB_PASS`, `INITIAL_ADMIN_PASSWORD` and S3 keys.
        Kept out of the Nix store.
      '';
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open the configured `APP_PORT` in the firewall.";
    };
  };

  config = lib.mkIf cfg.enable {
    services.squad-aegis.settings = {
      APP_PORT = lib.mkDefault 3113;
      APP_IN_CONTAINER = lib.mkDefault false;
      STORAGE_TYPE = lib.mkDefault "local";
      # Persisted under the systemd StateDirectory.
      STORAGE_LOCAL_PATH = lib.mkDefault "/var/lib/squad-aegis/storage";
    };

    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [
      cfg.settings.APP_PORT
    ];

    systemd.services.squad-aegis = {
      description = "Squad Aegis control panel";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];

      inherit environment;

      serviceConfig = {
        ExecStart = lib.getExe cfg.package;
        EnvironmentFile = lib.mkIf (cfg.environmentFile != null) cfg.environmentFile;

        DynamicUser = true;
        StateDirectory = "squad-aegis";
        WorkingDirectory = "/var/lib/squad-aegis";
        Restart = "on-failure";
        RestartSec = 5;

        # Hardening.
        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        PrivateDevices = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        RestrictAddressFamilies = [
          "AF_INET"
          "AF_INET6"
          "AF_UNIX"
        ];
        RestrictNamespaces = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";
        SystemCallFilter = [
          "@system-service"
          "~@privileged"
        ];
      };
    };
  };

  meta.maintainers = [ ];
}
