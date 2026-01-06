"use client";

import {
    Calendar,
    Download,
    ExternalLink,
    Package2,
    Search,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import type { PackageInfo } from "@/lib/types";
import { BugReportDialog } from "./bug-report-dialog";
import { useAPIClient } from "./lumine-provider";

interface PackageTableProps {
    packages: PackageInfo[];
    repo?: string;
    arch?: string;
}

export function PackageTable({
    packages: initialPackages,
    repo,
    arch,
}: PackageTableProps) {
    const [packages, setPackages] = useState<PackageInfo[]>(initialPackages);
    const [searchQuery, setSearchQuery] = useState("");
    const api = useAPIClient();

    const handleSearch = (e: React.ChangeEvent<HTMLInputElement>) => {
        const query = e.target.value;
        setSearchQuery(query);

        if (!query.trim()) {
            setPackages(initialPackages);
            return;
        }

        const filtered = initialPackages.filter(
            (pkg) =>
                pkg.pkgname.toLowerCase().includes(query.toLowerCase()) ||
                pkg.pkgdesc.toLowerCase().includes(query.toLowerCase()),
        );

        setPackages(filtered);
    };

    const formatDate = (timestamp: number) => {
        const date = new Date(timestamp * 1000);
        return new Intl.DateTimeFormat("ja-JP", {
            year: "numeric",
            month: "short",
            day: "numeric",
        }).format(date);
    };

    return (
        <div className="space-y-6">
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                <div>
                    <h2 className="text-2xl md:text-3xl font-bold text-primary">
                        パッケージ一覧
                    </h2>
                    <p className="text-sm text-muted-foreground mt-1">
                        {packages.length} 個のパッケージ
                    </p>
                </div>

                <div className="relative w-full sm:w-80">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                        type="text"
                        placeholder="パッケージを検索..."
                        value={searchQuery}
                        onChange={handleSearch}
                        className="pl-10 bg-card/50 border-border/50 focus:border-primary transition-colors"
                    />
                </div>
            </div>

            {packages.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-20">
                    <div className="p-6 rounded-2xl bg-card border border-border shadow-sm">
                        <Package2 className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                        <p className="text-center text-muted-foreground">
                            パッケージが見つかりませんでした
                        </p>
                    </div>
                </div>
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {packages.map((pkg) => {
                        return (
                            <Card
                                key={pkg.pkgname}
                                className="border-border/50 hover:border-primary/50 transition-all duration-300"
                            >
                                <CardContent className="p-6 space-y-4">
                                    <div className="flex items-start justify-between gap-3">
                                        <div className="flex-1 min-w-0">
                                            <Link
                                                href={
                                                    repo && arch
                                                        ? `/package?repo=${encodeURIComponent(repo)}&arch=${encodeURIComponent(arch)}&pkgbase=${encodeURIComponent(pkg.pkgbase)}`
                                                        : "#"
                                                }
                                                className="block"
                                            >
                                                <h3 className="font-semibold text-lg truncate group-hover:text-primary transition-colors flex items-center gap-2">
                                                    <Package2 className="h-4 w-4 shrink-0" />
                                                    {pkg.pkgname}
                                                </h3>
                                            </Link>
                                            <Badge
                                                variant="secondary"
                                                className="mt-2 text-xs"
                                            >
                                                v{pkg.pkgver}
                                            </Badge>
                                        </div>
                                    </div>

                                    <p className="text-sm text-muted-foreground line-clamp-2 min-h-10">
                                        {pkg.pkgdesc}
                                    </p>

                                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                                        <Calendar className="h-3 w-3" />
                                        <span>{formatDate(pkg.builddate)}</span>
                                    </div>

                                    <div className="flex items-center gap-2 pt-2 border-t border-border/50">
                                        <BugReportDialog packageInfo={pkg} />
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            className="flex-1 gap-2 hover:bg-secondary/10 hover:text-secondary hover:border-secondary/50 transition-colors"
                                            onClick={() => {
                                                if (!repo || !arch) return;
                                                const downloadUrl =
                                                    api.endpoints.repoFile(
                                                        repo,
                                                        arch,
                                                        `${pkg.pkgname}-${pkg.pkgver}.pkg.tar.zst`,
                                                    );
                                                window.open(
                                                    downloadUrl,
                                                    "_blank",
                                                );
                                            }}
                                        >
                                            <Download className="h-3 w-3" />
                                            ダウンロード
                                        </Button>
                                        <Link
                                            href={
                                                repo && arch
                                                    ? `/package?repo=${encodeURIComponent(repo)}&arch=${encodeURIComponent(arch)}&pkgbase=${encodeURIComponent(pkg.pkgbase)}`
                                                    : "#"
                                            }
                                        >
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="hover:bg-primary/10 hover:text-primary transition-colors"
                                            >
                                                <ExternalLink className="h-4 w-4" />
                                            </Button>
                                        </Link>
                                    </div>
                                </CardContent>
                            </Card>
                        );
                    })}
                </div>
            )}
        </div>
    );
}
