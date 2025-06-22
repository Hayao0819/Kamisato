"use client";
import { useAPIClient } from "@/components/lumine-provider";
import { Button } from "@/components/ui/button";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableRow,
} from "@/components/ui/table";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

interface PackageDetail {
    pkgname: string;
    pkgbase: string;
    pkgver: string;
    pkgdesc: string;
    url: string;
    builddate: number;
    packager: string;
    size: number;
    arch: string;
    license: string[];
    replaces: string[];
    group: string[];
    conflict: string[];
    provides: string[];
    backup: string[];
    depend: string[];
    optdepend: string[];
    makedepend: string[];
    checkdepend: string[];
    xdata: Record<string, string>;
    pkgtype: string;
}

export default function ClientPackageDetailPage() {
    const searchParams = useSearchParams();
    const repo = searchParams.get("repo") || "";
    const arch = searchParams.get("arch") || "";
    const pkgbase = searchParams.get("pkgbase") || "";
    const [pkg, setPkg] = useState<PackageDetail | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const api = useAPIClient();

    useEffect(() => {
        if (!api.endpoints.executable) return;
        if (!repo || !arch || !pkgbase) {
            setError("URLパラメータが不足しています");
            setLoading(false);
            return;
        }
        const fetchDetail = async () => {
            setLoading(true);
            setError(null);
            try {
                const data = await api.fetchPackageDetail(repo, arch, pkgbase);
                setPkg(data);
            } catch (e: unknown) {
                if (e instanceof Error) {
                    setError(e.message);
                } else {
                    setError(String(e));
                }
            } finally {
                setLoading(false);
            }
        };
        fetchDetail();
    }, [repo, arch, pkgbase, api.endpoints.executable, api.fetchPackageDetail]);

    if (loading) return <div className="p-8 text-center">読み込み中...</div>;
    if (error)
        return <div className="p-8 text-center text-red-500">{error}</div>;
    if (!pkg) return null;

    return (
        <div className="max-w-2xl mx-auto py-8 px-4">
            <div className="mb-4 flex items-center gap-2">
                <Link href="/">
                    <Button variant="outline">一覧に戻る</Button>
                </Link>
                <span className="text-lg font-bold">{pkg.pkgname}</span>
                <span className="text-sm text-muted-foreground">
                    ({pkg.arch})
                </span>
            </div>
            <div className="mb-4">
                <div className="text-xl font-bold mb-1">
                    {pkg.pkgbase} {pkg.pkgver}
                </div>
                <div className="mb-2 text-muted-foreground">{pkg.pkgdesc}</div>
                {pkg.url && (
                    <a
                        href={pkg.url}
                        className="text-blue-600 hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        公式サイト
                    </a>
                )}
            </div>
            <Table className="border rounded-md bg-background">
                <TableBody>
                    <TableRow>
                        <TableHead className="w-32">パッケージ名</TableHead>
                        <TableCell>{pkg.pkgname}</TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>バージョン</TableHead>
                        <TableCell>{pkg.pkgver}</TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>アーキテクチャ</TableHead>
                        <TableCell>{pkg.arch}</TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>パッケージャ</TableHead>
                        <TableCell>{pkg.packager}</TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>ビルド日</TableHead>
                        <TableCell>
                            {new Date(pkg.builddate * 1000).toLocaleString()}
                        </TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>サイズ</TableHead>
                        <TableCell>{(pkg.size / 1024).toFixed(1)} KB</TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>ライセンス</TableHead>
                        <TableCell>{pkg.license.join(", ")}</TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>依存関係</TableHead>
                        <TableCell>
                            {pkg.depend.length ? pkg.depend.join(", ") : "なし"}
                        </TableCell>
                    </TableRow>
                    <TableRow>
                        <TableHead>提供</TableHead>
                        <TableCell>
                            {pkg.provides.length
                                ? pkg.provides.join(", ")
                                : "なし"}
                        </TableCell>
                    </TableRow>
                </TableBody>
            </Table>
        </div>
    );
}
