"use client";

import { HelloStatus } from "@/components/hello-status";
import { PackageTable } from "@/components/package-table";
import { RepoArchSelector } from "@/components/repo-arch-selector";
import { Button } from "@/components/ui/button";
import { getAllPkgsEndpoint } from "@/lib/api";
import type { PackageInfo, PacmanPkgsResponse } from "@/lib/types";
import { ServerIcon } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

export default function Home() {
    const [selectedRepo, setSelectedRepo] = useState<string | null>(null);
    const [selectedArch, setSelectedArch] = useState<string | null>(null);
    const [packages, setPackages] = useState<PackageInfo[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleRepoArchSelect = (repo: string, arch: string) => {
        setSelectedRepo(repo);
        setSelectedArch(arch);
    };

    useEffect(() => {
        if (selectedRepo && selectedArch) {
            const fetchPackages = async () => {
                setLoading(true);
                setError(null);
                try {
                    const res = await fetch(
                        getAllPkgsEndpoint(selectedRepo, selectedArch),
                    );
                    if (!res.ok) {
                        throw new Error(
                            `Failed to fetch packages: ${res.statusText}`,
                        );
                    }
                    const data: PacmanPkgsResponse = await res.json();
                    if (!Array.isArray(data.packages)) {
                        console.error(
                            "Fetched data.packages is not an array:",
                            data.packages,
                        );
                        setPackages([]);
                    } else {
                        setPackages(data.packages);
                    }
                } catch (err: any) {
                    console.error("Failed to fetch packages:", err);
                    setError(err.message);
                    setPackages([]);
                } finally {
                    setLoading(false);
                }
            };

            fetchPackages();
        } else {
            setPackages([]);
        }
    }, [selectedRepo, selectedArch]);

    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6">
            <header className="mb-6 sm:mb-8">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-4">
                    <div>
                        <h1 className="text-2xl sm:text-3xl font-bold">
                            Lumine - Arch Linux パッケージリポジトリ
                        </h1>
                        <HelloStatus />
                    </div>
                    <Link href="/server-status">
                        <Button variant="outline" className="w-full sm:w-auto">
                            <ServerIcon className="h-4 w-4 mr-2" />
                            サーバーステータス
                        </Button>
                    </Link>
                </div>
                <p className="text-sm sm:text-base text-muted-foreground">
                    LumineはAyatoバックエンドを利用したArch
                    Linux向けの非公式パッケージリポジトリWebフロントエンドです。パッケージの検索・ダウンロードが可能です。
                </p>
                <div className="mt-4">
                    <RepoArchSelector onSelect={handleRepoArchSelect} />
                </div>
            </header>

            <main>
                {loading && (
                    <div className="text-center">Loading packages...</div>
                )}
                {error && (
                    <div className="text-center text-red-500">
                        Error: {error}
                    </div>
                )}
                {!loading && !error && <PackageTable packages={packages} />}
            </main>

            <footer className="mt-8 sm:mt-12 text-center text-xs sm:text-sm text-muted-foreground py-4">
                <p>© 2025 山田ハヤオ / Kamisato Project</p>
            </footer>
        </div>
    );
}
