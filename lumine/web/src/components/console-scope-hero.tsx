"use client";

import { Package, Search } from "lucide-react";
import { useRouter } from "next/navigation";
import type React from "react";
import { useEffect, useRef, useState } from "react";
import { PageContainer } from "@/components/page-container";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useRepoArch } from "@/hooks/use-repo-arch";

// The calm centered landing for "/": the Lumine mark, the repo/arch scope
// selects fused into one dominant search box, and nothing else. Submitting
// navigates seamlessly to the /packages category with the scope + keyword
// carried in the URL.
export function ScopeHero() {
    const {
        selectedRepo,
        setSelectedRepo,
        selectedArch,
        setSelectedArch,
        repos,
        arches,
    } = useRepoArch();
    const router = useRouter();
    const [keyword, setKeyword] = useState("");
    const inputRef = useRef<HTMLInputElement>(null);

    const scopeReady = Boolean(selectedRepo && selectedArch);

    // Autofocus the search box, and bind a global "/" shortcut to focus it.
    useEffect(() => {
        inputRef.current?.focus();
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== "/") return;
            const el = document.activeElement;
            const tag = el?.tagName;
            if (tag === "INPUT" || tag === "TEXTAREA") return;
            e.preventDefault();
            inputRef.current?.focus();
        };
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    }, []);

    const buildHref = (withKeyword: boolean) => {
        const params = new URLSearchParams();
        if (selectedRepo) params.set("repo", selectedRepo);
        if (selectedArch) params.set("arch", selectedArch);
        const q = keyword.trim();
        if (withKeyword && q) params.set("q", q);
        return `/packages?${params.toString()}`;
    };

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!scopeReady) return;
        router.push(buildHref(true));
    };

    const showAll = () => {
        if (!scopeReady) return;
        router.push(buildHref(false));
    };

    return (
        <PageContainer measure="full">
            <div className="flex min-h-[60vh] flex-col items-center justify-center text-center">
                <div className="w-full max-w-2xl">
                    <div className="mb-5 inline-flex h-14 w-14 items-center justify-center rounded-md bg-primary">
                        <Package className="h-8 w-8 text-primary-foreground" />
                    </div>
                    <h1 className="text-[32px] font-bold tracking-tight">
                        Lumine
                    </h1>
                    <p className="mx-auto mt-2 max-w-md text-[15px] leading-relaxed text-muted-foreground">
                        Ayaka が支える Arch Linux
                        向けパッケージリポジトリのフロントエンドです。
                    </p>
                    <p className="mt-4 text-[15px] text-muted-foreground">
                        {scopeReady ? (
                            <span className="tabular-nums">
                                <span className="font-medium text-foreground">
                                    {selectedRepo}
                                </span>
                                {" / "}
                                <span className="font-medium text-foreground">
                                    {selectedArch}
                                </span>
                                {" のパッケージを検索"}
                            </span>
                        ) : (
                            "リポジトリとアーキテクチャを選んで検索を始めましょう"
                        )}
                    </p>

                    <form onSubmit={handleSubmit} className="mt-8">
                        <div className="flex h-14 items-stretch overflow-hidden rounded-md border border-input bg-card focus-within:ring-2 focus-within:ring-ring">
                            <Select
                                value={selectedRepo || undefined}
                                onValueChange={setSelectedRepo}
                                disabled={repos.length === 0}
                            >
                                <SelectTrigger className="h-full w-32 shrink-0 rounded-none border-0 border-r border-input bg-muted/50 px-3 text-[14px] focus:ring-0 sm:w-36">
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
                            <Select
                                value={selectedArch || undefined}
                                onValueChange={setSelectedArch}
                                disabled={arches.length === 0}
                            >
                                <SelectTrigger className="h-full w-28 shrink-0 rounded-none border-0 border-r border-input bg-muted/50 px-3 text-[14px] focus:ring-0 sm:w-32">
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
                            <div className="relative flex-1">
                                <Search className="pointer-events-none absolute left-3.5 top-1/2 h-5 w-5 -translate-y-1/2 text-muted-foreground" />
                                <input
                                    ref={inputRef}
                                    type="text"
                                    placeholder="パッケージ名・説明で検索..."
                                    value={keyword}
                                    onChange={(e) => setKeyword(e.target.value)}
                                    className="h-full w-full bg-transparent pl-11 pr-4 text-[18px] focus:outline-none"
                                />
                            </div>
                            <button
                                type="submit"
                                disabled={!scopeReady}
                                aria-label="検索"
                                className="flex h-full shrink-0 items-center gap-1.5 border-l border-input bg-primary px-5 text-[15px] font-medium text-primary-foreground hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                            >
                                <Search className="h-4 w-4" />
                                <span className="hidden sm:inline">検索</span>
                            </button>
                        </div>

                        <div className="mt-4 flex items-center justify-center gap-3 text-[13px] text-muted-foreground">
                            {scopeReady ? (
                                <button
                                    type="button"
                                    onClick={showAll}
                                    className="text-link hover:underline"
                                >
                                    すべて表示
                                </button>
                            ) : (
                                <span>
                                    リポジトリとアーキテクチャを選ぶと検索できます
                                </span>
                            )}
                            <span className="hidden items-center gap-1 sm:inline-flex">
                                <kbd className="rounded-sm border border-border bg-muted px-1.5 py-0.5 font-mono text-[11px]">
                                    /
                                </kbd>
                                で検索
                            </span>
                        </div>
                    </form>
                </div>
            </div>
        </PageContainer>
    );
}
