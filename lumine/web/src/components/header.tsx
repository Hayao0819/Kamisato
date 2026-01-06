"use client";
import { Hammer, Moon, Package, ServerIcon, Sun, Upload } from "lucide-react";
import Link from "next/link";
import { useTheme } from "next-themes";
import { useEffect, useRef, useState } from "react";
import { LoginDialog } from "@/components/login-dialog";
import { ServerConfigDialog } from "@/components/server-config-dialog";
import { Button } from "@/components/ui/button";
import { useAuth } from "./auth-provider";
import { useAPIClient } from "./lumine-provider";

export function Header() {
    const [status, setStatus] = useState<
        "success" | "error" | "loading" | "unset"
    >("loading");
    const api = useAPIClient();
    const { authRequired } = useAuth();
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
        // Check every 1 hour (3600000ms)
        const timer = setInterval(check, 3600000);
        return () => {
            ignore = true;
            clearInterval(timer);
        };
    }, [api.endpoints.executable]);

    return (
        <header className="w-full sticky top-0 z-50 backdrop-blur-xl border-b border-border bg-background/95">
            <div className="container mx-auto">
                <div className="flex items-center justify-between py-4 px-4 md:px-6">
                    <Link href="/" className="flex items-center gap-3 group">
                        <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center group-hover:bg-primary/15 transition-colors">
                            <Package className="h-6 w-6 text-primary" />
                        </div>
                        <div>
                            <h1 className="text-xl md:text-2xl font-bold text-primary">
                                Lumine Repository
                            </h1>
                            <p className="text-xs text-muted-foreground hidden sm:block">
                                Arch Linux Package Repository
                            </p>
                        </div>
                    </Link>

                    <nav className="flex items-center gap-2">
                        <Link href="/upload" className="hidden md:block">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="gap-2 hover:bg-primary/10 hover:text-primary transition-colors"
                            >
                                <Upload className="h-4 w-4" />
                                アップロード
                            </Button>
                        </Link>

                        <Link href="/builds" className="hidden md:block">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="gap-2 hover:bg-primary/10 hover:text-primary transition-colors"
                            >
                                <Hammer className="h-4 w-4" />
                                ビルド
                            </Button>
                        </Link>

                        <Link href="/about" className="hidden md:block">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="gap-2 hover:bg-primary/10 hover:text-primary transition-colors"
                            >
                                このサイトについて
                            </Button>
                        </Link>

                        <Link href="/server-status">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="gap-2 hover:bg-secondary/10 hover:text-secondary transition-colors"
                            >
                                <ServerIcon className="h-4 w-4" />
                                <span className="hidden sm:inline">
                                    サーバー
                                </span>
                                <span
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
                                className="hover:bg-accent/10 hover:text-accent transition-colors"
                            >
                                {theme === "dark" ? (
                                    <Sun className="h-5 w-5" />
                                ) : (
                                    <Moon className="h-5 w-5" />
                                )}
                            </Button>
                        )}

                        {authRequired && <LoginDialog />}
                        <ServerConfigDialog />
                    </nav>
                </div>
            </div>
        </header>
    );
}
