# ayato package-repository server (NixOS service). See ../module.nix for the
# secrets/settings convention shared by all Kamisato services.
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
    types
    getExe
    ;
  shared = import ./_lib.nix { inherit lib; };
  json = pkgs.formats.json { };
  cfg = config.services.ayato;
in
{
  options.services.ayato = {
    enable = mkEnableOption "the ayato package-repository server";
    package = mkPackageOption pkgs "ayato" { };
    settings = mkOption {
      inherit (json) type;
      default = { };
      example = {
        port = 8080;
        store = {
          dbtype = "badgerdb";
          storagetype = "localfs";
          badgerdb = "/var/lib/ayato/badger";
          localrepodir = "/var/lib/ayato/repo";
        };
        auth.github.client_id = "Iv1.xxxx";
      };
      description = ''
        ayato_config.json contents (schema: internal/conf/ayato.go). Point local
        store paths under /var/lib/ayato (the StateDirectory); secrets go in
        environmentFile.
      '';
    };
    environmentFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = "systemd EnvironmentFile holding AYATO_* secrets.";
    };
    openFirewall = mkOption {
      type = types.bool;
      default = false;
      description = "Open settings.port in the firewall.";
    };
  };

  config = mkIf cfg.enable (
    let
      configFile = json.generate "ayato_config.json" cfg.settings;
    in
    {
      systemd.services.ayato = {
        description = "ayato package-repository server";
        wantedBy = [ "multi-user.target" ];
        after = [ "network-online.target" ];
        wants = [ "network-online.target" ];
        serviceConfig =
          shared.commonHardening
          // {
            ExecStart = "${getExe cfg.package} --config ${configFile}";
            DynamicUser = true;
            StateDirectory = "ayato";
            Restart = "on-failure";
            RestartSec = 5;
          }
          // shared.envFileAttrs cfg.environmentFile;
      };
      networking.firewall.allowedTCPPorts = mkIf (cfg.openFirewall && cfg.settings ? port) [
        cfg.settings.port
      ];
    }
  );
}
