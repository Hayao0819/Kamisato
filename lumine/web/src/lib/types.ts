// Canonical API shapes are generated from the Go truth types with tygo so the
// client cannot drift from the server. Regenerate with `pnpm gen:types`
// (config: tygo.yaml). Only client-only refinements live here by hand.
import type { PacmanPkgs } from "./generated/ayato_domain";
import type {
    BuildJob,
    BuildRequest as GeneratedBuildRequest,
    BuildStats,
    GitSource,
} from "./generated/miko_domain";
import type { PKGINFO } from "./generated/raiou";

export type PackageInfo = PKGINFO;
export type PacmanPkgsResponse = PacmanPkgs;
export type { BuildStats, GitSource };

// miko serializes install_pkgs and gpg_key unconditionally, but a client may
// omit them on submit and let the server apply defaults; relax just those two.
export type BuildRequest = Omit<
    GeneratedBuildRequest,
    "install_pkgs" | "gpg_key"
> &
    Partial<Pick<GeneratedBuildRequest, "install_pkgs" | "gpg_key">>;

// Closed set the UI switches on; narrows miko's open JobStatus string.
export type JobStatus =
    | "queued"
    | "running"
    | "success"
    | "failed"
    | "cancelled";

// Job mirrors miko's BuildJob (proxied through ayato), with status narrowed.
export type Job = Omit<BuildJob, "status"> & { status: JobStatus };
