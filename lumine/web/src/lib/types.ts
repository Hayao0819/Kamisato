export interface PackageInfo {
    pkgname: string;
    pkgbase: string;
    pkgver: string;
    pkgdesc: string;
    url: string;
    builddate: number;
    packager: string;
    size: number;
    arch: string;
    license: string[];
    replaces: string[];
    group: string[];
    conflict: string[];
    provides: string[];
    backup: string[];
    depend: string[];
    optdepend: string[];
    makedepend: string[];
    checkdepend: string[];
    xdata: { [key: string]: string };
    pkgtype: string;
}

export interface PacmanPkgsResponse {
    name: string;
    arch: string;
    packages: PackageInfo[];
}

export type JobStatus =
    | "queued"
    | "running"
    | "success"
    | "failed"
    | "cancelled";

export interface GitSource {
    url: string;
    ref?: string;
    subdir?: string;
}

export interface BuildRequest {
    repo: string;
    arch: string;
    git?: GitSource;
    pkgbuild?: string;
    files?: { [name: string]: string };
    install_pkgs?: string[];
    gpg_key?: string;
    timeout?: number;
}

// Job mirrors miko's BuildJob serialized form (proxied through ayato).
export interface Job {
    id: string;
    repo: string;
    arch: string;
    status: JobStatus;
    logs: string;
    err?: string;
    packages?: string[];
    created_at: string;
    started_at?: string;
    ended_at?: string;
    retries?: number;
}

// BuildStats mirrors miko's runtime stats (proxied through ayato).
export interface BuildStats {
    workers: number;
    queue_length: number;
    running: number;
    counts: Record<string, number>;
    total: number;
    success_rate: number;
    uptime_sec: number;
}

// TODO: Add other relevant type definitions from the backend
