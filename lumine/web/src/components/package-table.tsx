"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table";
import { useMobile } from "@/hooks/use-mobile";
// import { getRepoFileEndpoint } from "@/lib/api";
import type { PackageInfo } from "@/lib/types";
import {
    Calendar,
    Download,
    Info,
    MoreVertical,
    PackageIcon,
} from "lucide-react";
import { useState } from "react";
import { BugReportDialog } from "./bug-report-dialog";
import { useAPIClient } from "./lumine-provider";
import { SearchBar } from "./search-bar";

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
    const isMobile = useMobile();
    const api = useAPIClient();

    const handleSearch = (query: string) => {
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
        const date = new Date(timestamp * 1000); // Convert Unix timestamp to milliseconds
        return date.toLocaleDateString(); // Format as a localized date string
    };

    // モバイル用のカードビュー
    const renderMobileView = () => {
        return (
            <div className="grid grid-cols-1 gap-4">
                {packages.length === 0 ? (
                    <div className="text-center p-8 border rounded-md">
                        パッケージが見つかりませんでした
                    </div>
                ) : (
                    packages.map((pkg) => (
                        <Card key={pkg.pkgname}>
                            <CardContent className="pt-6">
                                <div className="flex items-start justify-between">
                                    <div className="flex items-center">
                                        <PackageIcon className="h-4 w-4 mr-2 text-muted-foreground" />
                                        <a
                                            href={
                                                repo && arch
                                                    ? `/package?repo=${encodeURIComponent(repo)}&arch=${encodeURIComponent(arch)}&pkgbase=${encodeURIComponent(pkg.pkgbase)}`
                                                    : undefined
                                            }
                                            className="font-medium hover:underline"
                                        >
                                            {pkg.pkgname}
                                        </a>
                                    </div>
                                    <Badge variant="outline">
                                        {pkg.pkgver}
                                    </Badge>
                                </div>
                                <p className="text-sm text-muted-foreground mt-2">
                                    {pkg.pkgdesc}
                                </p>
                                <div className="flex items-center text-xs text-muted-foreground mt-3">
                                    <Calendar className="h-3 w-3 mr-1" />
                                    {formatDate(pkg.builddate)}
                                </div>
                            </CardContent>
                            <CardFooter className="flex justify-between pt-0">
                                <BugReportDialog packageInfo={pkg} />
                                <DropdownMenu>
                                    <DropdownMenuTrigger asChild>
                                        <Button variant="ghost" size="icon">
                                            <MoreVertical className="h-4 w-4" />
                                            <span className="sr-only">
                                                メニュー
                                            </span>
                                        </Button>
                                    </DropdownMenuTrigger>
                                    <DropdownMenuContent align="end">
                                        <DropdownMenuLabel>
                                            アクション
                                        </DropdownMenuLabel>
                                        <DropdownMenuSeparator />
                                        <DropdownMenuItem asChild>
                                            <a
                                                href={
                                                    repo && arch
                                                        ? `/package?repo=${encodeURIComponent(repo)}&arch=${encodeURIComponent(arch)}&pkgbase=${encodeURIComponent(pkg.pkgbase)}`
                                                        : undefined
                                                }
                                                className="flex items-center"
                                            >
                                                <Info className="h-4 w-4 mr-2" />
                                                詳細を表示
                                            </a>
                                        </DropdownMenuItem>
                                        <DropdownMenuItem
                                            onSelect={() => {
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
                                            <Download className="h-4 w-4 mr-2" />
                                            ダウンロード
                                        </DropdownMenuItem>
                                    </DropdownMenuContent>
                                </DropdownMenu>
                            </CardFooter>
                        </Card>
                    ))
                )}
            </div>
        );
    };

    // デスクトップ用のテーブルビュー
    const renderDesktopView = () => {
        return (
            <div className="rounded-md border">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-[300px]">
                                パッケージ名
                            </TableHead>
                            <TableHead>バージョン</TableHead>
                            <TableHead>説明</TableHead>
                            <TableHead>最終更新日</TableHead>
                            <TableHead className="text-right">
                                アクション
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {packages.length === 0 ? (
                            <TableRow>
                                <TableCell
                                    colSpan={5}
                                    className="h-24 text-center"
                                >
                                    パッケージが見つかりませんでした
                                </TableCell>
                            </TableRow>
                        ) : (
                            packages.map((pkg) => (
                                <TableRow key={pkg.pkgname}>
                                    <TableCell className="font-medium">
                                        <div className="flex items-center">
                                            <PackageIcon className="h-4 w-4 mr-2 text-muted-foreground" />
                                            <a
                                                href={
                                                    repo && arch
                                                        ? `/package?repo=${encodeURIComponent(repo)}&arch=${encodeURIComponent(arch)}&pkgbase=${encodeURIComponent(pkg.pkgbase)}`
                                                        : undefined
                                                }
                                                className="hover:underline"
                                            >
                                                {pkg.pkgname}
                                            </a>
                                        </div>
                                    </TableCell>
                                    <TableCell>{pkg.pkgver}</TableCell>
                                    <TableCell>{pkg.pkgdesc}</TableCell>
                                    <TableCell>
                                        {formatDate(pkg.builddate)}
                                    </TableCell>
                                    <TableCell className="text-right">
                                        <div className="flex justify-end gap-2">
                                            <BugReportDialog
                                                packageInfo={pkg}
                                            />

                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button
                                                        variant="ghost"
                                                        size="icon"
                                                    >
                                                        <MoreVertical className="h-4 w-4" />
                                                        <span className="sr-only">
                                                            メニュー
                                                        </span>
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuLabel>
                                                        アクション
                                                    </DropdownMenuLabel>
                                                    <DropdownMenuSeparator />
                                                    <DropdownMenuItem asChild>
                                                        <a
                                                            href={
                                                                repo && arch
                                                                    ? `/package?repo=${encodeURIComponent(repo)}&arch=${encodeURIComponent(arch)}&pkgbase=${encodeURIComponent(pkg.pkgbase)}`
                                                                    : undefined
                                                            }
                                                            className="flex items-center"
                                                        >
                                                            <Info className="h-4 w-4 mr-2" />
                                                            詳細を表示
                                                        </a>
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        onSelect={() => {
                                                            if (!repo || !arch)
                                                                return;
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
                                                        <Download className="h-4 w-4 mr-2" />
                                                        ダウンロード
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>
            </div>
        );
    };

    return (
        <div className="space-y-4">
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                <h2 className="text-2xl font-bold tracking-tight">
                    パッケージ一覧
                </h2>
                <SearchBar onSearch={handleSearch} />
            </div>

            {isMobile ? renderMobileView() : renderDesktopView()}
        </div>
    );
}
