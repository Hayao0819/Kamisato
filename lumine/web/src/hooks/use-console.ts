"use client";

import { atom, useAtom } from "jotai";
import type { PackageInfo } from "@/lib/types";

// Shared console state so the persistent sidebar (facets) and the main list
// view operate on the same loaded package set.
const packagesAtom = atom<PackageInfo[]>([]);
const groupFilterAtom = atom<string | null>(null);
const pkgtypeFilterAtom = atom<string | null>(null);
const keywordAtom = atom<string>("");
const mobileNavOpenAtom = atom<boolean>(false);

export type SortKey = "pkgname" | "pkgver" | "arch" | "size" | "builddate";
export type SortDir = "asc" | "desc";

export const SORT_KEYS: SortKey[] = [
    "pkgname",
    "pkgver",
    "arch",
    "size",
    "builddate",
];
export const PAGE_SIZES = [50, 100, 250];
export const DEFAULT_PAGE_SIZE = 50;

// View state for the /packages table. Kept in shared atoms (rather than local
// component state) so the sidebar facets and the URL-sync layer in
// packages-client stay in lockstep with the table itself.
const sortKeyAtom = atom<SortKey>("pkgname");
const sortDirAtom = atom<SortDir>("asc");
const pageAtom = atom<number>(1);
const pageSizeAtom = atom<number>(DEFAULT_PAGE_SIZE);

export function useConsolePackages() {
    return useAtom(packagesAtom);
}

export function useConsoleFilters() {
    const [group, setGroup] = useAtom(groupFilterAtom);
    const [pkgtype, setPkgtype] = useAtom(pkgtypeFilterAtom);
    const [keyword, setKeyword] = useAtom(keywordAtom);
    return { group, setGroup, pkgtype, setPkgtype, keyword, setKeyword };
}

export function useConsoleView() {
    const [sortKey, setSortKey] = useAtom(sortKeyAtom);
    const [sortDir, setSortDir] = useAtom(sortDirAtom);
    const [page, setPage] = useAtom(pageAtom);
    const [pageSize, setPageSize] = useAtom(pageSizeAtom);
    return {
        sortKey,
        setSortKey,
        sortDir,
        setSortDir,
        page,
        setPage,
        pageSize,
        setPageSize,
    };
}

export function useMobileNav() {
    return useAtom(mobileNavOpenAtom);
}

export interface FacetValue {
    value: string;
    count: number;
}

export function computeFacet(
    packages: PackageInfo[],
    pick: (p: PackageInfo) => string[],
): FacetValue[] {
    const counts = new Map<string, number>();
    for (const pkg of packages) {
        for (const v of pick(pkg)) {
            if (!v) continue;
            counts.set(v, (counts.get(v) ?? 0) + 1);
        }
    }
    return Array.from(counts.entries())
        .map(([value, count]) => ({ value, count }))
        .sort((a, b) => b.count - a.count || a.value.localeCompare(b.value));
}
