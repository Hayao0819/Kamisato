"use client";
import { Hammer, Moon, Package, ServerIcon, Sun, Upload } from "lucide-react";
import Link from "next/link";
import { useTheme } from "next-themes";
import { useEffect, useRef, useState } from "react";
import { useCanMutate } from "@/components/auth-gate";
import { LoginDialog } from "@/components/login-dialog";
import { Button } from "@/components/ui/button";
import { useAPIClient } from "./lumine-provider";

export function Header() {
    const [status, setStatus] = useState<
        "success" | "error" | "loading" | "unset"
    >("loading");
    const api = useAPIClient();
    const canMutate = useCanMutate();
    const { theme, setTheme } = useTheme();
    const [mounted, setMounted] = useState(false);
    const apiRef = useRef(api);

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

    const statusLabel =
        status === "success"
            ? "オンライン"
            : status === "error"
              ? "オフライン"
              : status === "unset"
                ? "未設定"
                : "確認中";

    return (
        <header className="w-full sticky top-0 z-50 arch-titlebar shadow-sm">
            <div className="container mx-auto">
                <div className="flex items-center justify-between gap-2 py-2.5 px-4 md:px-6">
                    <Link
                        href="/"
                        className="flex items-center gap-2.5 group shrink-0"
                    >
                        <div className="w-8 h-8 rounded-sm bg-primary flex items-center justify-center">
                            <Package className="h-5 w-5 text-primary-foreground" />
                        </div>
                        <div className="leading-tight">
                            <h1 className="text-lg md:text-xl font-bold text-arch-bar-foreground tracking-tight">
                                Lumine
                                <span className="text-primary">
                                    {" "}
                                    Repository
                                </span>
                            </h1>
                            <p className="text-[11px] text-arch-bar-foreground/60 hidden sm:block">
                                Arch Linux Package Repository
                            </p>
                        </div>
                    </Link>

                    <nav className="flex items-center gap-0.5 md:gap-1">
                        {canMutate && (
                            <Link href="/upload" className="hidden md:block">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="gap-1.5 rounded-sm text-arch-bar-foreground/85 hover:bg-white/10 hover:text-primary"
                                >
                                    <Upload className="h-4 w-4" />
                                    アップロード
                                </Button>
                            </Link>
                        )}

                        <Link href="/builds" className="hidden md:block">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="gap-1.5 rounded-sm text-arch-bar-foreground/85 hover:bg-white/10 hover:text-primary"
                            >
                                <Hammer className="h-4 w-4" />
                                ビルド
                            </Button>
                        </Link>

                        <Link href="/about" className="hidden md:block">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="rounded-sm text-arch-bar-foreground/85 hover:bg-white/10 hover:text-primary"
                            >
                                このサイトについて
                            </Button>
                        </Link>

                        <Link href="/server-status">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="gap-1.5 rounded-sm text-arch-bar-foreground/85 hover:bg-white/10 hover:text-primary"
                            >
                                <ServerIcon className="h-4 w-4" />
                                <span
                                    title={statusLabel}
                                    className={
                                        status === "success"
                                            ? "h-2 w-2 rounded-full bg-emerald-500 animate-pulse"
                                            : status === "error"
                                              ? "h-2 w-2 rounded-full bg-red-500"
                                              : status === "unset"
                                                ? "h-2 w-2 rounded-full bg-yellow-500"
                                                : "h-2 w-2 rounded-full bg-gray-400 animate-pulse"
                                    }
                                />
                                <span className="hidden sm:inline">
                                    {statusLabel}
                                </span>
                            </Button>
                        </Link>

                        {mounted && (
                            <Button
                                variant="ghost"
                                size="icon"
                                onClick={() =>
                                    setTheme(
                                        theme === "dark" ? "light" : "dark",
                                    )
                                }
                                className="rounded-sm text-arch-bar-foreground/85 hover:bg-white/10 hover:text-primary"
                            >
                                {theme === "dark" ? (
                                    <Sun className="h-5 w-5" />
                                ) : (
                                    <Moon className="h-5 w-5" />
                                )}
                            </Button>
                        )}

                        {mounted && <LoginDialog />}
                    </nav>
                </div>
            </div>
        </header>
    );
}
