"use client";

import { HelloStatus } from "@/components/hello-status";
import { PackageTable } from "@/components/package-table";
import { RepoArchSelector } from "@/components/repo-arch-selector";
import { Button } from "@/components/ui/button";
import { Footer } from "@/components/footer";
import { ArrowRight } from "lucide-react";
// import { getAllPkgsEndpoint } from "@/lib/api";
import type { PackageInfo, PacmanPkgsResponse } from "@/lib/types";
import { ServerIcon } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useToast } from "@/hooks/use-toast";
import { useAPIClient } from "@/components/lumine-provider";

export default function Home() {
    const [selectedRepo, setSelectedRepo] = useState<string | null>(null);
    const [selectedArch, setSelectedArch] = useState<string | null>(null);
    const [packages, setPackages] = useState<PackageInfo[]>([]);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const { toast } = useToast();

    const handleRepoArchSelect = (repo: string, arch: string) => {
        setSelectedRepo(repo);
        setSelectedArch(arch);
    };

    const api = useAPIClient();

    useEffect(() => {
        if (!api.endpoints.executable) return;
        if (selectedRepo && selectedArch) {
            const fetchPackages = async () => {
                setLoading(true);
                setError(null);
                try {
                    const data: PacmanPkgsResponse = await api.fetchAllPkgs(selectedRepo, selectedArch);
                    if (!Array.isArray(data.packages)) {
                        console.error("Fetched data.packages is not an array:", data.packages);
                        setPackages([]);
                    } else {
                        setPackages(data.packages);
                    }
                } catch (err: any) {
                    console.error("Failed to fetch packages:", err);
                    setError(err.message);
                    setPackages([]);
                    toast({
                        title: "パッケージ取得エラー",
                        description: err.message || "API通信に失敗しました。",
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
    }, [selectedRepo, selectedArch, toast, api.endpoints.executable]);

    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6 flex flex-col">
            <div className="mt-4 mb-6 sm:mb-8">
                <RepoArchSelector onSelect={handleRepoArchSelect} />
            </div>

            {loading && <div className="text-center">Loading packages...</div>}
            {error && (
                <div className="text-center text-red-500">Error: {error}</div>
            )}
            {!loading && !error && selectedRepo && selectedArch && (
                <PackageTable
                    packages={packages}
                    repo={selectedRepo}
                    arch={selectedArch}
                />
            )}
            {!loading && !error && (!selectedRepo || !selectedArch) && (
                <div className="text-center text-muted-foreground py-12">
                    上部のリポジトリ・アーキテクチャを選択してください。
                </div>
            )}
        </div>
    );
}
