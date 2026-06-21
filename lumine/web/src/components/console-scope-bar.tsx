"use client";

import { Package, Search } from "lucide-react";
import { useRouter } from "next/navigation";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useRepoArch } from "@/hooks/use-repo-arch";

// Compact scope (repo + arch) bar pinned to the top of the main area so the
// active scope is always visible and switchable away from the sidebar.
export function ScopeBar() {
    const {
        selectedRepo,
        setSelectedRepo,
        selectedArch,
        setSelectedArch,
        repos,
        arches,
    } = useRepoArch();
    const router = useRouter();

    const scopeReady = Boolean(selectedRepo && selectedArch);
    const browseHref = () => {
        const params = new URLSearchParams();
        if (selectedRepo) params.set("repo", selectedRepo);
        if (selectedArch) params.set("arch", selectedArch);
        return `/packages?${params.toString()}`;
    };

    return (
        <div className="sticky top-0 z-20 border-b border-border bg-background/95 px-4 py-2.5 backdrop-blur sm:px-6 lg:px-8">
            <div className="flex flex-wrap items-center gap-2">
                <span className="text-[12px] font-medium text-muted-foreground">
                    スコープ
                </span>
                <Select
                    value={selectedRepo || undefined}
                    onValueChange={setSelectedRepo}
                    disabled={repos.length === 0}
                >
                    <SelectTrigger className="h-8 w-40 rounded-sm bg-card text-[14px]">
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
                    onValueChange={setSelectedArch}
                    disabled={arches.length === 0}
                >
                    <SelectTrigger className="h-8 w-36 rounded-sm bg-card text-[14px]">
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

                <div className="ml-auto flex items-center gap-1.5">
                    <button
                        type="button"
                        onClick={() => router.push("/")}
                        className="inline-flex h-8 items-center gap-1.5 rounded-sm border border-border px-3 text-[14px] hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                        <Search className="h-4 w-4" />
                        検索
                    </button>
                    <button
                        type="button"
                        onClick={() => router.push(browseHref())}
                        disabled={!scopeReady}
                        className="inline-flex h-8 items-center gap-1.5 rounded-sm border border-border px-3 text-[14px] hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                    >
                        <Package className="h-4 w-4" />
                        パッケージ
                    </button>
                </div>
            </div>
        </div>
    );
}
