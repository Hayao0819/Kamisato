"use client";
import { ServerConfigDialog } from "@/components/server-config-dialog";
import { Button } from "@/components/ui/button";
import { ServerIcon } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

export function Header() {
    const [status, setStatus] = useState<'success' | 'error' | 'loading' | 'unset'>("loading");
    useEffect(() => {
        let ignore = false;
        const check = async () => {
            // localStorageからAPI URLを取得
            let apiBase = undefined;
            if (typeof window !== "undefined") {
                apiBase = window.localStorage.getItem("lumine_api_base_url")?.trim();
            }
            if (!apiBase) {
                setStatus("unset");
                return;
            }
            setStatus("loading");
            try {
                const res = await fetch(apiBase + "/api/unstable/hello");
                if (!ignore) setStatus(res.ok ? "success" : "error");
            } catch {
                if (!ignore) setStatus("error");
            }
        };
        check();
        const timer = setInterval(check, 10000);
        return () => { ignore = true; clearInterval(timer); };
    }, []);
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
                        LumineはAyakaバックエンドを利用したArch Linux向けの非公式パッケージリポジトリWebフロントエンドです。
                    </p>
                </div>
                <div className="flex gap-2 w-full sm:w-auto items-center justify-end">
                    <Link href="/about">
                        <Button variant="ghost" className="w-full sm:w-auto">
                            このサイトについて
                        </Button>
                    </Link>
                    <Link href="/server-status">
                        <Button variant="outline" className="w-full sm:w-auto flex items-center">
                            <ServerIcon className="h-4 w-4 mr-2" />
                            <span>サーバー</span>
                            <span className={
                                status === "success"
                                    ? "ml-2 px-2 py-0.5 rounded bg-green-500 text-white text-xs"
                                    : status === "error"
                                        ? "ml-2 px-2 py-0.5 rounded bg-red-500 text-white text-xs"
                                        : status === "unset"
                                            ? "ml-2 px-2 py-0.5 rounded bg-yellow-500 text-white text-xs"
                                            : "ml-2 px-2 py-0.5 rounded bg-gray-400 text-white text-xs animate-pulse"
                            }>
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
