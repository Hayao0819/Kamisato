"use client";
import { ServerConfigDialog } from "@/components/server-config-dialog";
import { Button } from "@/components/ui/button";
import { ServerIcon } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, useRef } from "react";
import { useAPIClient } from "./lumine-provider";
// import { getHelloEndpoint } from "@/lib/api";

export function Header() {
    const [status, setStatus] = useState<
        "success" | "error" | "loading" | "unset"
    >("loading");
    const api = useAPIClient();
    const apiRef = useRef(api);
    useEffect(() => {
        apiRef.current = api;
    }, [api]);
    useEffect(() => {
        if (!api.endpoints.executable) return;
        let ignore = false;
        const check = async () => {
            setStatus("loading");
            try {
                const res = await apiRef.current.fetchHello();
                // 418もsuccess扱い
                if (!ignore)
                    setStatus(
                        res.ok || res.status === 418 ? "success" : "error",
                    );
            } catch {
                if (!ignore) setStatus("error");
            }
        };
        check();
        const timer = setInterval(check, 10000);
        return () => {
            ignore = true;
            clearInterval(timer);
        };
    }, [api.endpoints.executable]);
    return (
        <header className="w-full bg-background/80 border-b sticky top-0 z-40 backdrop-blur">
            <div className="container mx-auto flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 py-3 px-4">
                <div>
                    <Link href="/">
                        <h1 className="text-2xl sm:text-3xl font-bold cursor-pointer">
                            Lumine - Arch Linux パッケージリポジトリ
                        </h1>
                    </Link>
                    <p className="text-xs sm:text-sm text-muted-foreground mt-1">
                        LumineはAyakaバックエンドを利用したArch
                        Linux向けの非公式パッケージリポジトリWebフロントエンドです。
                    </p>
                </div>
                <div className="flex gap-2 w-full sm:w-auto items-center justify-end">
                    <Link href="/about">
                        <Button variant="ghost" className="w-full sm:w-auto">
                            このサイトについて
                        </Button>
                    </Link>
                    <Link href="/server-status">
                        <Button
                            variant="outline"
                            className="w-full sm:w-auto flex items-center"
                        >
                            <ServerIcon className="h-4 w-4 mr-2" />
                            <span>サーバー</span>
                            <span
                                className={
                                    status === "success"
                                        ? "ml-2 px-2 py-0.5 rounded bg-green-500 text-white text-xs"
                                        : status === "error"
                                          ? "ml-2 px-2 py-0.5 rounded bg-red-500 text-white text-xs"
                                          : status === "unset"
                                            ? "ml-2 px-2 py-0.5 rounded bg-yellow-500 text-white text-xs"
                                            : "ml-2 px-2 py-0.5 rounded bg-gray-400 text-white text-xs animate-pulse"
                                }
                            >
                                {status === "success"
                                    ? "オンライン"
                                    : status === "error"
                                      ? "オフライン"
                                      : status === "unset"
                                        ? "未設定"
                                        : "確認中..."}
                            </span>
                        </Button>
                    </Link>
                    <ServerConfigDialog />
                </div>
            </div>
        </header>
    );
}
