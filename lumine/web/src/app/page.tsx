"use client";
import { Package2, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";
import { useAPIClient } from "@/components/lumine-provider";
import { PackageTable } from "@/components/package-table";
import { RepoArchSelector } from "@/components/repo-arch-selector";
import { useRepoArch } from "@/hooks/use-repo-arch";
import { useToast } from "@/hooks/use-toast";
import type { PackageInfo, PacmanPkgsResponse } from "@/lib/types";

export default function Home() {
    const { selectedRepo, selectedArch } = useRepoArch();
    const [packages, setPackages] = useState<PackageInfo[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const { toast } = useToast();

    const api = useAPIClient();

    useEffect(() => {
        if (!api.endpoints.executable) return;
        if (selectedRepo && selectedArch) {
            const fetchPackages = async () => {
                setLoading(true);
                setError(null);
                try {
                    const data: PacmanPkgsResponse = await api.fetchAllPkgs(
                        selectedRepo,
                        selectedArch,
                    );
                    if (!Array.isArray(data.packages)) {
                        console.error(
                            "Fetched data.packages is not an array:",
                            data.packages,
                        );
                        setPackages([]);
                    } else {
                        setPackages(data.packages);
                    }
                } catch (err: unknown) {
                    console.error("Failed to fetch packages:", err);
                    let message = "API通信に失敗しました。";
                    if (err instanceof Error) {
                        message = err.message;
                    }
                    setError(message);
                    setPackages([]);
                    toast({
                        title: "パッケージ取得エラー",
                        description: message,
                        variant: "destructive",
                    });
                } finally {
                    setLoading(false);
                }
            };
            fetchPackages();
        } else {
            setPackages([]);
        }
    }, [
        selectedRepo,
        selectedArch,
        toast,
        api.endpoints.executable,
        api.fetchAllPkgs,
    ]);

    return (
        <div className="flex flex-col min-h-[calc(100vh-5rem)]">
            <section className="bg-muted/30 border-b border-border">
                <div className="container mx-auto px-4 sm:px-6 py-16 md:py-24">
                    <div className="max-w-3xl mx-auto text-center space-y-6">
                        <div className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary/10 border border-primary/20">
                            <Sparkles className="h-4 w-4 text-primary" />
                            <span className="text-sm font-medium text-primary">
                                Arch Linux Repository
                            </span>
                        </div>

                        <h1 className="text-4xl md:text-6xl font-bold tracking-tight text-primary">
                            Lumine Repository
                        </h1>

                        <p className="text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto leading-relaxed">
                            Ayakaバックエンドを利用した
                            <br className="hidden sm:block" />
                            Arch Linux向けパッケージリポジトリ
                        </p>

                        <div className="flex flex-wrap items-center justify-center gap-3 pt-4">
                            <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-card border border-border">
                                <div className="h-2 w-2 rounded-full bg-primary" />
                                <span className="text-sm text-muted-foreground">
                                    Ayaka CLI
                                </span>
                            </div>
                            <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-card border border-border">
                                <div className="h-2 w-2 rounded-full bg-secondary" />
                                <span className="text-sm text-muted-foreground">
                                    Ayato Backend
                                </span>
                            </div>
                            <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-card border border-border">
                                <div className="h-2 w-2 rounded-full bg-accent" />
                                <span className="text-sm text-muted-foreground">
                                    Lumine Web
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <div className="container mx-auto px-4 sm:px-6 py-8 flex-1">
                <div className="mb-8">
                    <RepoArchSelector />
                </div>

                {loading && (
                    <div className="flex flex-col items-center justify-center py-20 space-y-4">
                        <Package2 className="h-12 w-12 text-primary animate-pulse" />
                        <p className="text-muted-foreground">
                            パッケージを読み込み中...
                        </p>
                    </div>
                )}

                {error && (
                    <div className="flex flex-col items-center justify-center py-20 space-y-4">
                        <div className="p-4 rounded-xl bg-destructive/10 border border-destructive/20">
                            <p className="text-destructive font-medium">
                                エラー: {error}
                            </p>
                        </div>
                    </div>
                )}

                {!loading && !error && selectedRepo && selectedArch && (
                    <PackageTable
                        packages={packages}
                        repo={selectedRepo}
                        arch={selectedArch}
                    />
                )}

                {!loading && !error && (!selectedRepo || !selectedArch) && (
                    <div className="flex flex-col items-center justify-center py-20 space-y-4">
                        <div className="p-6 rounded-2xl bg-card border border-border shadow-sm">
                            <Package2 className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                            <p className="text-center text-muted-foreground">
                                リポジトリとアーキテクチャを選択して
                                <br />
                                パッケージを表示
                            </p>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
