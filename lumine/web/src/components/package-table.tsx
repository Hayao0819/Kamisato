"use client";

import { ArrowDown, ArrowUp, ArrowUpDown, Download, Package2 } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";
import { Pagination } from "@/components/pagination";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { PAGE_SIZES, type SortDir, type SortKey } from "@/hooks/use-console";
import type { PackageInfo } from "@/lib/types";
import { cn, formatBuildDate, formatBytes } from "@/lib/utils";
import { useAPIClient } from "./lumine-provider";

interface PackageTableProps {
    packages: PackageInfo[];
    repo?: string;
    arch?: string;
    keyword: string;
    group: string | null;
    pkgtype: string | null;
    sortKey: SortKey;
    sortDir: SortDir;
    page: number;
    pageSize: number;
    onSetGroup: (v: string | null) => void;
    onSetPkgtype: (v: string | null) => void;
    onSetSortKey: (v: SortKey) => void;
    onSetSortDir: (v: SortDir) => void;
    onSetPage: (v: number) => void;
    onSetPageSize: (v: number) => void;
}

const COLUMNS: {
    key: SortKey | null;
    label: string;
    title: string;
    className?: string;
}[] = [
    {
        key: "pkgname",
        label: "パッケージ名",
        title: "パッケージ名で並び替え",
    },
    { key: "pkgver", label: "バージョン", title: "バージョンで並び替え" },
    { key: null, label: "説明", title: "説明" },
    { key: "arch", label: "アーキ", title: "アーキテクチャで並び替え" },
    { key: null, label: "グループ", title: "グループ" },
    {
        key: "size",
        label: "サイズ",
        title: "サイズで並び替え",
        className: "text-right",
    },
    { key: "builddate", label: "ビルド日", title: "ビルド日で並び替え" },
    { key: null, label: "操作", title: "操作", className: "text-right" },
];

export function PackageTable({
    packages,
    repo,
    arch,
    keyword,
    group,
    pkgtype,
    sortKey,
    sortDir,
    page,
    pageSize,
    onSetGroup,
    onSetPkgtype,
    onSetSortKey,
    onSetSortDir,
    onSetPage,
    onSetPageSize,
}: PackageTableProps) {
    const api = useAPIClient();

    const detailHref = (pkg: PackageInfo) =>
        repo && arch
            ? `/package?repo=${encodeURIComponent(repo)}&arch=${encodeURIComponent(arch)}&pkgbase=${encodeURIComponent(pkg.pkgbase)}`
            : "#";

    const filtered = useMemo(() => {
        const kw = keyword.trim().toLowerCase();
        return packages.filter((pkg) => {
            if (
                kw &&
                !pkg.pkgname.toLowerCase().includes(kw) &&
                !pkg.pkgdesc.toLowerCase().includes(kw)
            )
                return false;
            if (group && !(pkg.group ?? []).includes(group)) return false;
            if (pkgtype && pkg.pkgtype !== pkgtype) return false;
            return true;
        });
    }, [packages, keyword, group, pkgtype]);

    const sorted = useMemo(() => {
        const arr = [...filtered];
        arr.sort((a, b) => {
            let cmp = 0;
            if (sortKey === "size" || sortKey === "builddate") {
                cmp = a[sortKey] - b[sortKey];
            } else {
                cmp = String(a[sortKey]).localeCompare(String(b[sortKey]));
            }
            return sortDir === "asc" ? cmp : -cmp;
        });
        return arr;
    }, [filtered, sortKey, sortDir]);

    const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize));
    const current = Math.min(Math.max(1, page), pageCount);
    const start = (current - 1) * pageSize;
    const rows = sorted.slice(start, start + pageSize);

    const toggleSort = (key: SortKey) => {
        if (key === sortKey) {
            onSetSortDir(sortDir === "asc" ? "desc" : "asc");
        } else {
            onSetSortKey(key);
            onSetSortDir(
                key === "builddate" || key === "size" ? "desc" : "asc",
            );
        }
    };

    const handleDownload = (pkg: PackageInfo) => {
        if (!repo || !arch) return;
        const url = api.endpoints.repoFile(
            repo,
            arch,
            `${pkg.pkgname}-${pkg.pkgver}.pkg.tar.zst`,
        );
        window.open(url, "_blank");
    };

    return (
        <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
                <Select
                    value={sortKey}
                    onValueChange={(v) => onSetSortKey(v as SortKey)}
                >
                    <SelectTrigger className="h-9 w-44 rounded-sm text-[14px]">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="pkgname">名前順</SelectItem>
                        <SelectItem value="pkgver">バージョン順</SelectItem>
                        <SelectItem value="arch">アーキ順</SelectItem>
                        <SelectItem value="size">サイズ順</SelectItem>
                        <SelectItem value="builddate">ビルド日順</SelectItem>
                    </SelectContent>
                </Select>

                <button
                    type="button"
                    onClick={() =>
                        onSetSortDir(sortDir === "asc" ? "desc" : "asc")
                    }
                    title="昇順／降順を切り替え"
                    className="inline-flex h-9 items-center gap-1.5 rounded-sm border border-border px-3 text-[14px] hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                    {sortDir === "asc" ? (
                        <ArrowUp className="h-4 w-4" />
                    ) : (
                        <ArrowDown className="h-4 w-4" />
                    )}
                    {sortDir === "asc" ? "昇順" : "降順"}
                </button>

                <Select
                    value={String(pageSize)}
                    onValueChange={(v) => onSetPageSize(Number(v))}
                >
                    <SelectTrigger className="h-9 w-28 rounded-sm text-[14px]">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {PAGE_SIZES.map((s) => (
                            <SelectItem key={s} value={String(s)}>
                                {s} 件
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </div>

            <div className="flex flex-wrap items-center gap-2 text-[14px] text-muted-foreground">
                <span className="tabular-nums">
                    {sorted.length} 件 ・ ページ {current} / {pageCount}
                </span>
                {(group || pkgtype || keyword) && (
                    <span className="flex flex-wrap items-center gap-1.5">
                        {keyword && (
                            <span className="inline-flex items-center gap-1 rounded-sm border border-border bg-muted px-1.5 py-0.5 text-[12px]">
                                検索: {keyword}
                            </span>
                        )}
                        {group && (
                            <span className="inline-flex items-center gap-1 rounded-sm border border-border bg-muted px-1.5 py-0.5 text-[12px]">
                                グループ: {group}
                                <button
                                    type="button"
                                    onClick={() => onSetGroup(null)}
                                    className="text-muted-foreground hover:text-foreground"
                                >
                                    ×
                                </button>
                            </span>
                        )}
                        {pkgtype && (
                            <span className="inline-flex items-center gap-1 rounded-sm border border-border bg-muted px-1.5 py-0.5 text-[12px]">
                                種別: {pkgtype}
                                <button
                                    type="button"
                                    onClick={() => onSetPkgtype(null)}
                                    className="text-muted-foreground hover:text-foreground"
                                >
                                    ×
                                </button>
                            </span>
                        )}
                    </span>
                )}
            </div>

            {sorted.length === 0 ? (
                <div className="rounded-sm border border-border bg-card py-14 text-center">
                    <Package2 className="mx-auto mb-3 h-10 w-10 text-muted-foreground" />
                    <p className="text-[15px] text-muted-foreground">
                        条件に一致するパッケージはありません
                    </p>
                </div>
            ) : (
                <div className="overflow-x-auto rounded-sm border border-border bg-card">
                    <table className="arch-table w-full text-[15px]">
                        <thead>
                            <tr>
                                {COLUMNS.map((col) => {
                                    const active =
                                        col.key !== null && col.key === sortKey;
                                    return (
                                        <th
                                            key={col.label}
                                            title={col.title}
                                            className={cn(
                                                "whitespace-nowrap px-4 py-2.5 text-[14px] font-semibold",
                                                col.className,
                                            )}
                                        >
                                            {col.key ? (
                                                <button
                                                    type="button"
                                                    onClick={() =>
                                                        toggleSort(
                                                            col.key as SortKey,
                                                        )
                                                    }
                                                    className={cn(
                                                        "inline-flex items-center gap-1 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                                                        col.className ===
                                                            "text-right" &&
                                                            "flex-row-reverse",
                                                    )}
                                                >
                                                    {col.label}
                                                    {active ? (
                                                        sortDir === "asc" ? (
                                                            <ArrowUp className="h-3.5 w-3.5" />
                                                        ) : (
                                                            <ArrowDown className="h-3.5 w-3.5" />
                                                        )
                                                    ) : (
                                                        <ArrowUpDown className="h-3.5 w-3.5 opacity-40" />
                                                    )}
                                                </button>
                                            ) : (
                                                col.label
                                            )}
                                        </th>
                                    );
                                })}
                            </tr>
                        </thead>
                        <tbody>
                            {rows.map((pkg) => {
                                const groups = pkg.group ?? [];
                                return (
                                    <tr key={pkg.pkgname} className="group">
                                        <td className="whitespace-nowrap px-4 py-3">
                                            <Link
                                                href={detailHref(pkg)}
                                                className="font-medium text-link hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                                            >
                                                {pkg.pkgname}
                                            </Link>
                                        </td>
                                        <td className="whitespace-nowrap px-4 py-3 font-mono text-[14px] tabular-nums text-muted-foreground">
                                            {pkg.pkgver}
                                        </td>
                                        <td className="max-w-md truncate px-4 py-3 text-muted-foreground">
                                            {pkg.pkgdesc}
                                        </td>
                                        <td className="whitespace-nowrap px-4 py-3 text-muted-foreground">
                                            {pkg.arch}
                                        </td>
                                        <td className="whitespace-nowrap px-4 py-3 text-muted-foreground">
                                            {groups.length === 0 ? (
                                                <span className="text-muted-foreground/50">
                                                    —
                                                </span>
                                            ) : (
                                                <span className="inline-flex items-center gap-1">
                                                    <button
                                                        type="button"
                                                        onClick={() =>
                                                            onSetGroup(
                                                                groups[0],
                                                            )
                                                        }
                                                        className="rounded-sm border border-border bg-muted px-1.5 py-0.5 text-[13px] hover:border-primary"
                                                    >
                                                        {groups[0]}
                                                    </button>
                                                    {groups.length > 1 && (
                                                        <span className="text-[12px]">
                                                            +{groups.length - 1}
                                                            ▾
                                                        </span>
                                                    )}
                                                </span>
                                            )}
                                        </td>
                                        <td className="whitespace-nowrap px-4 py-3 text-right tabular-nums text-muted-foreground">
                                            {formatBytes(pkg.size)}
                                        </td>
                                        <td className="whitespace-nowrap px-4 py-3 tabular-nums text-muted-foreground">
                                            {formatBuildDate(pkg.builddate)}
                                        </td>
                                        <td className="whitespace-nowrap px-4 py-3 text-right">
                                            <div className="inline-flex items-center justify-end gap-1 opacity-60 transition-opacity group-hover:opacity-100">
                                                <button
                                                    type="button"
                                                    onClick={() =>
                                                        handleDownload(pkg)
                                                    }
                                                    title="ダウンロード"
                                                    className="rounded-sm p-1.5 hover:bg-muted hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                                                >
                                                    <Download className="h-4 w-4" />
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
            )}

            {pageCount > 1 && (
                <div className="flex justify-center pt-1">
                    <Pagination
                        page={current}
                        pageCount={pageCount}
                        onPageChange={onSetPage}
                    />
                </div>
            )}
        </div>
    );
}
