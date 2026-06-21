import {
    DEFAULT_PAGE_SIZE,
    SORT_KEYS,
    type SortDir,
    type SortKey,
} from "@/hooks/use-console";

// Canonical query model for the /packages route. Kept in one place so the
// packages client and the sidebar facets agree on parsing and serialization.
export type PackagesQuery = {
    repo: string;
    arch: string;
    q: string;
    group: string | null;
    type: string | null;
    sort: SortKey;
    dir: SortDir;
    page: number;
    per: number;
};

export function parsePackagesQuery(
    params: URLSearchParams | ReadonlyURLSearchParams,
): PackagesQuery {
    const sortRaw = params.get("sort");
    const sort = SORT_KEYS.includes(sortRaw as SortKey)
        ? (sortRaw as SortKey)
        : "pkgname";
    const dir = params.get("dir") === "desc" ? "desc" : "asc";
    const page = Math.max(1, Number(params.get("page")) || 1);
    const per = Number(params.get("per")) || DEFAULT_PAGE_SIZE;
    return {
        repo: params.get("repo") || "",
        arch: params.get("arch") || "",
        q: params.get("q") || "",
        group: params.get("group") || null,
        type: params.get("type") || null,
        sort,
        dir,
        page,
        per,
    };
}

// Serialize a query, omitting defaults/unset params so shared URLs stay clean.
export function buildPackagesQuery(q: PackagesQuery): string {
    const params = new URLSearchParams();
    if (q.repo) params.set("repo", q.repo);
    if (q.arch) params.set("arch", q.arch);
    if (q.q) params.set("q", q.q);
    if (q.group) params.set("group", q.group);
    if (q.type) params.set("type", q.type);
    if (q.sort !== "pkgname") params.set("sort", q.sort);
    if (q.dir !== "asc") params.set("dir", q.dir);
    if (q.page > 1) params.set("page", String(q.page));
    if (q.per !== DEFAULT_PAGE_SIZE) params.set("per", String(q.per));
    return params.toString();
}

export function packagesHref(q: PackagesQuery): string {
    const qs = buildPackagesQuery(q);
    return qs ? `/packages?${qs}` : "/packages";
}

// Minimal structural type matching next/navigation's ReadonlyURLSearchParams
// for the .get() we rely on, without importing from the framework here.
interface ReadonlyURLSearchParams {
    get(name: string): string | null;
}
