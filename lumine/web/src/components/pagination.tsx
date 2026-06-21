"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";

interface PaginationProps {
    page: number;
    pageCount: number;
    onPageChange: (page: number) => void;
}

function pageWindow(page: number, pageCount: number): number[] {
    const span = 2;
    const start = Math.max(1, page - span);
    const end = Math.min(pageCount, page + span);
    const pages: number[] = [];
    for (let i = start; i <= end; i++) pages.push(i);
    return pages;
}

export function Pagination({ page, pageCount, onPageChange }: PaginationProps) {
    if (pageCount <= 1) return null;
    const pages = pageWindow(page, pageCount);

    const btn =
        "inline-flex h-8 min-w-8 items-center justify-center rounded-sm border border-border px-2 text-[14px] tabular-nums focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-40 disabled:cursor-not-allowed";

    return (
        <nav
            className="flex items-center gap-1"
            aria-label="ページネーション"
        >
            <button
                type="button"
                className={cn(btn, "hover:bg-muted")}
                onClick={() => onPageChange(page - 1)}
                disabled={page <= 1}
                title="前のページ"
            >
                <ChevronLeft className="h-4 w-4" />
            </button>

            {pages[0] > 1 && (
                <>
                    <button
                        type="button"
                        className={cn(btn, "hover:bg-muted")}
                        onClick={() => onPageChange(1)}
                    >
                        1
                    </button>
                    {pages[0] > 2 && (
                        <span className="px-1 text-muted-foreground">…</span>
                    )}
                </>
            )}

            {pages.map((p) => (
                <button
                    key={p}
                    type="button"
                    aria-current={p === page ? "page" : undefined}
                    className={cn(
                        btn,
                        p === page
                            ? "border-primary bg-primary text-primary-foreground"
                            : "hover:bg-muted",
                    )}
                    onClick={() => onPageChange(p)}
                >
                    {p}
                </button>
            ))}

            {pages[pages.length - 1] < pageCount && (
                <>
                    {pages[pages.length - 1] < pageCount - 1 && (
                        <span className="px-1 text-muted-foreground">…</span>
                    )}
                    <button
                        type="button"
                        className={cn(btn, "hover:bg-muted")}
                        onClick={() => onPageChange(pageCount)}
                    >
                        {pageCount}
                    </button>
                </>
            )}

            <button
                type="button"
                className={cn(btn, "hover:bg-muted")}
                onClick={() => onPageChange(page + 1)}
                disabled={page >= pageCount}
                title="次のページ"
            >
                <ChevronRight className="h-4 w-4" />
            </button>
        </nav>
    );
}
