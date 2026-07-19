// Canonical API shapes are generated from the Go truth types with tygo so the
// client cannot drift from the server. Regenerate with `pnpm gen:types`
// (config: tygo.yaml). Only client-only refinements live here by hand.
import type {
    Features,
    PacmanPackage,
    PacmanPkgs,
} from "./generated/ayato_domain";
import type {
    BuildJob,
    BuildStats,
    BuildRequest as GeneratedBuildRequest,
    GitSource,
} from "./generated/miko_domain";

export type PackageInfo = PacmanPackage;
export type PacmanPkgsResponse = PacmanPkgs;
export type { BuildStats, Features, GitSource };

// miko serializes install_pkgs unconditionally, but a client may omit it on
// submit and let the server apply defaults; relax just that field.
export type BuildRequest = Omit<GeneratedBuildRequest, "install_pkgs"> &
    Partial<Pick<GeneratedBuildRequest, "install_pkgs">>;

// Closed set the UI switches on; narrows miko's open JobStatus string.
export type JobStatus =
    | "queued"
    | "running"
    | "success"
    | "failed"
    | "cancelled";

// Job mirrors miko's BuildJob (proxied through ayato), with status narrowed.
export type Job = Omit<BuildJob, "status"> & { status: JobStatus };
