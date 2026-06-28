"use client";

import {
    Hammer,
    Info,
    Moon,
    Package,
    Search,
    ServerIcon,
    Sun,
    Upload,
    X,
} from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTheme } from "next-themes";
import { useEffect, useRef, useState } from "react";
import { useCanMutate } from "@/components/auth-gate";
import { LoginDialog } from "@/components/login-dialog";
import {
    computeFacet,
    useConsoleFilters,
    useConsolePackages,
    useMobileNav,
} from "@/hooks/use-console";
import { buildPackagesQuery, parsePackagesQuery } from "@/lib/packages-url";
import { useAPIClient } from "./lumine-provider";

const NAV = [
    { href: "/", label: "検索", icon: Search },
    { href: "/packages", label: "パッケージ", icon: Package },
    { href: "/builds", label: "ビルド", icon: Hammer },
    { href: "/upload", label: "アップロード", icon: Upload },
    { href: "/server-status", label: "サーバー状態", icon: ServerIcon },
    { href: "/about", label: "このサイトについて", icon: Info },
];

type ServerState = "success" | "error" | "loading" | "unset";

function StatusDot({ status }: { status: ServerState }) {
    const cls =
        status === "success"
            ? "bg-emerald-500 animate-pulse"
            : status === "error"
              ? "bg-red-500"
              : status === "unset"
                ? "bg-yellow-500"
                : "bg-gray-400 animate-pulse";
    return <span className={`h-2 w-2 rounded-full ${cls}`} />;
}

function FacetGroup({
    title,
    values,
    selected,
    onSelect,
}: {
    title: string;
    values: { value: string; count: number }[];
    selected: string | null;
    onSelect: (v: string | null) => void;
}) {
    if (values.length === 0) return null;
    return (
        <div className="space-y-1">
            <div className="flex items-center justify-between px-2">
                <span className="text-[11px] font-semibold uppercase tracking-wide text-sidebar-foreground/50">
                    {title}
                </span>
                {selected && (
                    <button
                        type="button"
                        onClick={() => onSelect(null)}
                        className="text-[11px] text-primary hover:underline"
                    >
                        解除
                    </button>
                )}
            </div>
            <ul className="max-h-56 overflow-y-auto">
                {values.map(({ value, count }) => {
                    const active = selected === value;
                    return (
                        <li key={value}>
                            <button
                                type="button"
                                onClick={() => onSelect(active ? null : value)}
                                className={`flex w-full items-center justify-between gap-2 border-l-2 py-1 pl-2 pr-2 text-left text-[14px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring ${
                                    active
                                        ? "border-primary bg-sidebar-accent text-sidebar-accent-foreground"
                                        : "border-transparent text-sidebar-foreground/85 hover:bg-sidebar-accent/60"
                                }`}
                            >
                                <span className="truncate">{value}</span>
                                <span className="shrink-0 text-[12px] tabular-nums text-sidebar-foreground/45">
                                    {count}
                                </span>
                            </button>
                        </li>
                    );
                })}
            </ul>
        </div>
    );
}

export function ConsoleSidebar() {
    const pathname = usePathname();
    const router = useRouter();
    const searchParams = useSearchParams();
    const api = useAPIClient();
    const apiRef = useRef(api);
    const canMutate = useCanMutate();
    const { theme, setTheme } = useTheme();
    const [mounted, setMounted] = useState(false);
    const [status, setStatus] = useState<ServerState>("loading");
    const [open, setOpen] = useMobileNav();

    const [packages] = useConsolePackages();
    const { group, pkgtype } = useConsoleFilters();

    const onPackages = pathname === "/packages";

    // Facet clicks reflect straight into the /packages URL (group/type), which
    // is the single source of truth the table renders from.
    const selectGroup = (v: string | null) => {
        const next = { ...parsePackagesQuery(searchParams), group: v, page: 1 };
        router.replace(`/packages?${buildPackagesQuery(next)}`, {
            scroll: false,
        });
    };
    const selectPkgtype = (v: string | null) => {
        const next = { ...parsePackagesQuery(searchParams), type: v, page: 1 };
        router.replace(`/packages?${buildPackagesQuery(next)}`, {
            scroll: false,
        });
    };

    const groupFacet = computeFacet(packages, (p) => p.group ?? []);
    const pkgtypeFacet = computeFacet(packages, (p) =>
        p.pkgtype ? [p.pkgtype] : [],
    );

    useEffect(() => {
        setMounted(true);
        apiRef.current = api;
    }, [api]);

    useEffect(() => {
        if (!api.endpoints.executable) return;
        let ignore = false;
        const check = async () => {
            setStatus("loading");
            try {
                const res = await apiRef.current.fetchHello();
                if (!ignore)
                    setStatus(
                        res.ok || res.status === 418 ? "success" : "error",
                    );
            } catch {
                if (!ignore) setStatus("error");
            }
        };
        check();
        const timer = setInterval(check, 3600000);
        return () => {
            ignore = true;
            clearInterval(timer);
        };
    }, [api.endpoints.executable]);

    // Close the mobile drawer on navigation.
    // biome-ignore lint/correctness/useExhaustiveDependencies: pathname is the navigation trigger, not read in the body.
    useEffect(() => {
        setOpen(false);
    }, [pathname, setOpen]);

    return (
        <>
            {open && (
                <button
                    type="button"
                    aria-label="サイドバーを閉じる"
                    className="fixed inset-0 z-40 bg-black/40 md:hidden"
                    onClick={() => setOpen(false)}
                />
            )}
            <aside
                className={`fixed inset-y-0 left-0 z-50 flex w-60 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-transform md:translate-x-0 ${
                    open ? "translate-x-0" : "-translate-x-full"
                }`}
            >
                <div className="flex items-center justify-between gap-2 border-b border-sidebar-border px-4 py-3.5">
                    <Link href="/" className="flex items-center gap-2.5">
                        <span className="flex h-8 w-8 items-center justify-center rounded-sm bg-primary">
                            <Package className="h-5 w-5 text-primary-foreground" />
                        </span>
                        <span className="leading-tight">
                            <span className="block text-[15px] font-bold tracking-tight">
                                Lumine
                            </span>
                            <span className="block text-[11px] text-sidebar-foreground/55">
                                Repo Console
                            </span>
                        </span>
                    </Link>
                    <button
                        type="button"
                        aria-label="閉じる"
                        className="rounded-sm p-1 text-sidebar-foreground/70 hover:bg-sidebar-accent md:hidden"
                        onClick={() => setOpen(false)}
                    >
                        <X className="h-5 w-5" />
                    </button>
                </div>

                <div className="flex-1 overflow-y-auto py-3">
                    <nav className="space-y-0.5 px-2">
                        {NAV.filter(
                            ({ href }) => href !== "/upload" || canMutate,
                        ).map(({ href, label, icon: Icon }) => {
                            const active =
                                href === "/"
                                    ? pathname === "/"
                                    : href === "/packages"
                                      ? pathname.startsWith("/package")
                                      : pathname.startsWith(href);
                            return (
                                <Link
                                    key={href}
                                    href={href}
                                    className={`flex items-center gap-2.5 rounded-sm border-l-2 px-2.5 py-2 text-[14px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring ${
                                        active
                                            ? "border-primary bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                                            : "border-transparent text-sidebar-foreground/80 hover:bg-sidebar-accent/60"
                                    }`}
                                >
                                    <Icon className="h-4 w-4 shrink-0" />
                                    {label}
                                </Link>
                            );
                        })}
                    </nav>

                    {onPackages && packages.length > 0 && (
                        <div className="mt-5 space-y-4 px-2">
                            <FacetGroup
                                title="グループ"
                                values={groupFacet}
                                selected={group}
                                onSelect={selectGroup}
                            />
                            <FacetGroup
                                title="種別"
                                values={pkgtypeFacet}
                                selected={pkgtype}
                                onSelect={selectPkgtype}
                            />
                        </div>
                    )}
                </div>

                <div className="flex items-center gap-1 border-t border-sidebar-border px-3 py-2.5">
                    <span className="flex items-center gap-1.5 text-[12px] text-sidebar-foreground/60">
                        <StatusDot status={status} />
                        サーバー
                    </span>
                    <span className="ml-auto flex items-center gap-1">
                        {mounted && (
                            <button
                                type="button"
                                aria-label="テーマ切替"
                                onClick={() =>
                                    setTheme(
                                        theme === "dark" ? "light" : "dark",
                                    )
                                }
                                className="rounded-sm p-1.5 text-sidebar-foreground/70 hover:bg-sidebar-accent"
                            >
                                {theme === "dark" ? (
                                    <Sun className="h-[18px] w-[18px]" />
                                ) : (
                                    <Moon className="h-[18px] w-[18px]" />
                                )}
                            </button>
                        )}
                        {mounted && <LoginDialog />}
                    </span>
                </div>
            </aside>
        </>
    );
}
