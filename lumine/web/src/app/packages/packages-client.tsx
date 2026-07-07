"use client";

import { Package2, Search } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import type React from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useAPIClient } from "@/components/lumine-provider";
import { PackageTable } from "@/components/package-table";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { PkgTableSkeleton } from "@/components/pkg-table-skeleton";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import {
    type SortDir,
    type SortKey,
    useConsoleFilters,
    useConsolePackages,
    useConsoleView,
} from "@/hooks/use-console";
import { useRepoArch } from "@/hooks/use-repo-arch";
import { useToast } from "@/hooks/use-toast";
import {
    buildPackagesQuery,
    type PackagesQuery,
    parsePackagesQuery,
} from "@/lib/packages-url";
import type { PacmanPkgsResponse } from "@/lib/types";

export default function PackagesClient() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const api = useAPIClient();
    const { toast } = useToast();

    const {
        selectedRepo,
        setScope,
        selectedArch,
        setSelectedRepo,
        setSelectedArch,
        repos,
        arches,
    } = useRepoArch();
    const [packages, setPackages] = useConsolePackages();
    const { group, setGroup, pkgtype, setPkgtype, keyword, setKeyword } =
        useConsoleFilters();
    const {
        sortKey,
        setSortKey,
        sortDir,
        setSortDir,
        page,
        setPage,
        pageSize,
        setPageSize,
    } = useConsoleView();

    const query = parsePackagesQuery(searchParams);
    const queryStr = searchParams.toString();

    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [refineDraft, setRefineDraft] = useState(query.q);

    // URL → state. The URL is the single source of truth; on any URL change we
    // hydrate the shared atoms (scope, filters, view) so the sidebar facets and
    // table render exactly what the address bar describes.
    // biome-ignore lint/correctness/useExhaustiveDependencies: queryStr is the single source of truth; the atom setters are stable.
    useEffect(() => {
        if (query.repo && query.arch) {
            setScope(query.repo, query.arch);
        } else if (query.repo) {
            setSelectedRepo(query.repo);
        }
        setKeyword(query.q);
        setGroup(query.group);
        setPkgtype(query.type);
        setSortKey(query.sort);
        setSortDir(query.dir);
        setPage(query.page);
        setPageSize(query.per);
    }, [queryStr]);

    // Fetch packages for the URL scope. Keyed on the resolved repo/arch so a
    // pure filter/sort/page change does not refetch.
    useEffect(() => {
        if (!api.endpoints.executable) return;
        const repo = query.repo;
        const arch = query.arch;
        if (!repo || !arch) {
            setPackages([]);
            return;
        }
        let ignore = false;
        const run = async () => {
            setLoading(true);
            setError(null);
            try {
                const data: PacmanPkgsResponse = await api.fetchAllPkgs(
                    repo,
                    arch,
                );
                if (ignore) return;
                if (!Array.isArray(data.packages)) {
                    console.error(
                        "Fetched data.packages is not an array:",
                        data.packages,
                    );
                    setPackages([]);
                } else {
                    setPackages(data.packages);
                }
            } catch (err: unknown) {
                if (ignore) return;
                console.error("Failed to fetch packages:", err);
                const message =
                    err instanceof Error
                        ? err.message
                        : "API通信に失敗しました。";
                setError(message);
                setPackages([]);
                toast({
                    title: "パッケージ取得エラー",
                    description: message,
                    variant: "destructive",
                });
            } finally {
                if (!ignore) setLoading(false);
            }
        };
        run();
        return () => {
            ignore = true;
        };
    }, [
        query.repo,
        query.arch,
        api.endpoints.executable,
        api.fetchAllPkgs,
        setPackages,
        toast,
    ]);

    // state → URL. Merge a partial change onto the current query and write it
    // back with replace() so the view stays shareable without flooding history.
    const updateUrl = useCallback(
        (patch: Partial<PackagesQuery>) => {
            const next: PackagesQuery = {
                ...parsePackagesQuery(searchParams),
                ...patch,
            };
            router.replace(`/packages?${buildPackagesQuery(next)}`, {
                scroll: false,
            });
        },
        [router, searchParams],
    );

    const onRepoChange = (repo: string) => {
        setSelectedRepo(repo);
        updateUrl({ repo, arch: "", group: null, type: null, page: 1 });
    };
    const onArchChange = (arch: string) => {
        setSelectedArch(arch);
        updateUrl({ arch, group: null, type: null, page: 1 });
    };

    const submitRefine = (e: React.FormEvent) => {
        e.preventDefault();
        updateUrl({ q: refineDraft.trim(), page: 1 });
    };

    const onSetGroup = (v: string | null) => updateUrl({ group: v, page: 1 });
    const onSetPkgtype = (v: string | null) => updateUrl({ type: v, page: 1 });
    const onSetSortKey = (v: SortKey) => updateUrl({ sort: v, page: 1 });
    const onSetSortDir = (v: SortDir) => updateUrl({ dir: v, page: 1 });
    const onSetPage = (v: number) => updateUrl({ page: v });
    const onSetPageSize = (v: number) => updateUrl({ per: v, page: 1 });

    // Reconcile the arch in the URL once the available arches for the chosen
    // repo resolve: if it's missing or invalid (e.g. after a repo switch),
    // canonicalize to the first available one so the scope becomes complete.
    useEffect(() => {
        if (!query.repo) return;
        if (arches.length === 0) return;
        if (query.arch && arches.includes(query.arch)) return;
        updateUrl({ arch: arches[0], group: null, type: null, page: 1 });
    }, [query.repo, query.arch, arches, updateUrl]);

    // Promote the auto-selected scope into the URL (the source of truth) when the
    // address bar is bare — e.g. arriving via the sidebar's plain /packages link —
    // so the view activates instead of staying on the "select repo/arch" prompt.
    // Guard on repo OR arch so this owns only the fully-empty URL, keeping its
    // domain disjoint from the URL→atoms hydration and the arch-canon effect above;
    // an overlapping domain would double-write and race an interactive repo switch.
    useEffect(() => {
        if (query.repo || query.arch) return;
        if (!selectedRepo || !selectedArch) return;
        updateUrl({ repo: selectedRepo, arch: selectedArch });
    }, [query.repo, query.arch, selectedRepo, selectedArch, updateUrl]);

    // Keep the refine box reflecting the active q after URL-driven hydration,
    // but don't clobber an in-progress edit before submit.
    const lastSyncedQ = useRef(query.q);
    useEffect(() => {
        if (lastSyncedQ.current !== query.q) {
            lastSyncedQ.current = query.q;
            setRefineDraft(query.q);
        }
    }, [query.q]);

    const scopeReady = Boolean(query.repo && query.arch);

    const scopeSummary = scopeReady
        ? `${query.repo} / ${query.arch} のパッケージを一覧`
        : "リポジトリとアーキテクチャを選んでパッケージを一覧";

    return (
        <PageContainer
            measure="full"
            header={
                <PageHeader title="パッケージ" description={scopeSummary} />
            }
        >
            <div className="space-y-5">
                <div className="flex flex-wrap items-center gap-2 rounded-sm border border-border bg-card px-3 py-2.5">
                    <span className="text-[12px] font-medium text-muted-foreground">
                        スコープ
                    </span>
                    <Select
                        value={selectedRepo || undefined}
                        onValueChange={onRepoChange}
                        disabled={repos.length === 0}
                    >
                        <SelectTrigger className="h-8 w-40 rounded-sm bg-background text-[14px]">
                            <SelectValue placeholder="リポジトリ" />
                        </SelectTrigger>
                        <SelectContent>
                            {repos.map((repo) => (
                                <SelectItem key={repo} value={repo}>
                                    {repo}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <span className="text-muted-foreground/50">/</span>
                    <Select
                        value={selectedArch || undefined}
                        onValueChange={onArchChange}
                        disabled={arches.length === 0}
                    >
                        <SelectTrigger className="h-8 w-36 rounded-sm bg-background text-[14px]">
                            <SelectValue placeholder="アーキ" />
                        </SelectTrigger>
                        <SelectContent>
                            {arches.map((arch) => (
                                <SelectItem key={arch} value={arch}>
                                    {arch}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>

                    <form
                        onSubmit={submitRefine}
                        className="relative ml-auto min-w-52 flex-1 sm:max-w-md"
                    >
                        <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <input
                            type="text"
                            placeholder="絞り込み検索..."
                            value={refineDraft}
                            onChange={(e) => setRefineDraft(e.target.value)}
                            className="h-8 w-full rounded-sm border border-input bg-background pl-8 pr-3 text-[14px] focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        />
                    </form>
                </div>

                {!scopeReady ? (
                    <div className="rounded-sm border border-border bg-card py-14 text-center">
                        <Package2 className="mx-auto mb-3 h-10 w-10 text-muted-foreground" />
                        <p className="text-[15px] text-muted-foreground">
                            リポジトリとアーキテクチャを選択してください
                        </p>
                    </div>
                ) : loading ? (
                    <PkgTableSkeleton rows={pageSize > 12 ? 12 : pageSize} />
                ) : error ? (
                    <div className="rounded-sm border border-destructive/40 bg-card p-3">
                        <p className="text-[15px] font-medium text-destructive">
                            エラー: {error}
                        </p>
                    </div>
                ) : (
                    <PackageTable
                        packages={packages}
                        repo={query.repo}
                        arch={query.arch}
                        keyword={keyword}
                        group={group}
                        pkgtype={pkgtype}
                        sortKey={sortKey}
                        sortDir={sortDir}
                        page={page}
                        pageSize={pageSize}
                        onSetGroup={onSetGroup}
                        onSetPkgtype={onSetPkgtype}
                        onSetSortKey={onSetSortKey}
                        onSetSortDir={onSetSortDir}
                        onSetPage={onSetPage}
                        onSetPageSize={onSetPageSize}
                    />
                )}
            </div>
        </PageContainer>
    );
}
