# Shared systemd hardening + helpers for the Kamisato service modules.
{ lib }:
let
  inherit (lib) optionalAttrs;
in
{
  # Hardening for the plain HTTP daemons (ayato, kayo); miko is excluded (Docker).
  commonHardening = {
    NoNewPrivileges = true;
    ProtectSystem = "strict";
    ProtectHome = true;
    PrivateTmp = true;
    ProtectKernelTunables = true;
    ProtectKernelModules = true;
    ProtectControlGroups = true;
    RestrictAddressFamilies = [
      "AF_INET"
      "AF_INET6"
      "AF_UNIX"
    ];
    RestrictSUIDSGID = true;
    RestrictNamespaces = true;
    LockPersonality = true;
    SystemCallArchitectures = "native";
  };

  envFileAttrs = file: optionalAttrs (file != null) { EnvironmentFile = file; };
}
