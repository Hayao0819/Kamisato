# miko build server (NixOS service). No DynamicUser and only a reduced hardening
# set: it needs the Docker socket and a stable data_dir owner.
{
  config,
  lib,
  pkgs,
  ...
}:
let
  inherit (lib)
    mkEnableOption
    mkPackageOption
    mkOption
    mkIf
    mkDefault
    types
    getExe
    optional
    ;
  shared = import ./_lib.nix { inherit lib; };
  json = pkgs.formats.json { };
  cfg = config.services.miko;
in
{
  options.services.miko = {
    enable = mkEnableOption "the miko build server";
    package = mkPackageOption pkgs "miko" { };
    user = mkOption {
      type = types.str;
      default = "miko";
      description = "User the service runs as.";
    };
    group = mkOption {
      type = types.str;
      default = "miko";
      description = "Primary group of the service user.";
    };
    settings = mkOption {
      inherit (json) type;
      default = { };
      example = {
        port = 8081;
        builder = {
          backend = "container";
          timeout = "30m";
          docker.image = "archlinux:latest";
        };
        concurrency = 2;
        ayato = {
          url = "https://repo.example.com";
          username = "miko";
        };
      };
      description = ''
        miko_config.json contents (schema: internal/conf/miko.go). The default
        builder backend "container" talks to the Docker daemon. Secrets (e.g.
        the ayato password via MIKO_AYATO_PASSWORD) belong in environmentFile.
      '';
    };
    environmentFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = "systemd EnvironmentFile holding MIKO_* secrets (e.g. MIKO_AYATO_PASSWORD).";
    };
    openFirewall = mkOption {
      type = types.bool;
      default = false;
      description = "Open the miko port in the firewall.";
    };
  };

  config = mkIf cfg.enable (
    let
      configFile = json.generate "miko_config.json" cfg.settings;
      backend = cfg.settings.builder.backend or (cfg.settings.executor or "container");
      needsDocker = backend == "container";
    in
    {
      users.users = mkIf (cfg.user == "miko") {
        miko = {
          isSystemUser = true;
          inherit (cfg) group;
          extraGroups = optional needsDocker "docker";
        };
      };
      users.groups = mkIf (cfg.group == "miko") { miko = { }; };

      virtualisation.docker.enable = mkIf needsDocker (mkDefault true);

      systemd.services.miko = {
        description = "miko build server";
        wantedBy = [ "multi-user.target" ];
        after = [ "network-online.target" ] ++ optional needsDocker "docker.service";
        wants = [ "network-online.target" ];
        # No DynamicUser: needs the docker socket + a stable data_dir owner.
        serviceConfig = {
          ExecStart = "${getExe cfg.package} --config ${configFile}";
          User = cfg.user;
          Group = cfg.group;
          StateDirectory = "miko";
          Restart = "on-failure";
          RestartSec = 5;
          NoNewPrivileges = true;
          PrivateTmp = true;
        }
        // shared.envFileAttrs cfg.environmentFile;
      };
      networking.firewall.allowedTCPPorts = mkIf cfg.openFirewall [
        (cfg.settings.port or 8081)
      ];
    }
  );
}
