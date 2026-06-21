"use client";
import { BugIcon, Download, ExternalLink } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { BugReportDialog } from "@/components/bug-report-dialog";
import { useAPIClient } from "@/components/lumine-provider";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { PkgDepSection } from "@/components/pkg-dep-section";
import { Button } from "@/components/ui/button";
import type { PackageInfo } from "@/lib/types";
import { formatBuildDate, formatBytes } from "@/lib/utils";

export default function ClientPackageDetailPage() {
    const searchParams = useSearchParams();
    const repo = searchParams.get("repo") || "";
    const arch = searchParams.get("arch") || "";
    const pkgbase = searchParams.get("pkgbase") || "";
    const [pkg, setPkg] = useState<PackageInfo | null>(null);
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

    const backToListParams = new URLSearchParams();
    if (repo) backToListParams.set("repo", repo);
    if (arch) backToListParams.set("arch", arch);
    const backToListHref = `/packages?${backToListParams.toString()}`;
    const packagesCrumb = { label: "パッケージ", href: backToListHref };

    if (loading) {
        return (
            <PageContainer
                measure="full"
                header={
                    <PageHeader
                        title="読み込み中…"
                        breadcrumbs={[packagesCrumb]}
                    />
                }
            >
                <p className="text-[15px] text-muted-foreground">
                    パッケージ情報を読み込み中...
                </p>
            </PageContainer>
        );
    }

    if (error || !pkg) {
        return (
            <PageContainer
                measure="full"
                header={
                    <PageHeader
                        title="パッケージを表示できません"
                        breadcrumbs={[packagesCrumb]}
                    />
                }
            >
                <div className="rounded-sm border border-destructive/40 bg-card p-3">
                    <p className="text-[15px] font-medium text-destructive">
                        {error ?? "パッケージが見つかりませんでした"}
                    </p>
                </div>
            </PageContainer>
        );
    }

    const handleDownload = () => {
        const url = api.endpoints.repoFile(
            repo,
            arch,
            `${pkg.pkgname}-${pkg.pkgver}.pkg.tar.zst`,
        );
        window.open(url, "_blank");
    };

    const meta: { label: string; value: React.ReactNode }[] = [
        { label: "pkgbase", value: pkg.pkgbase },
        {
            label: "バージョン",
            value: <span className="font-mono">{pkg.pkgver}</span>,
        },
        { label: "アーキテクチャ", value: pkg.arch },
        {
            label: "ライセンス",
            value: pkg.license?.length ? pkg.license.join(", ") : "—",
        },
        {
            label: "グループ",
            value: pkg.group?.length ? pkg.group.join(", ") : "—",
        },
        { label: "パッケージャ", value: pkg.packager || "—" },
        { label: "ビルド日", value: formatBuildDate(pkg.builddate) },
        { label: "サイズ", value: formatBytes(pkg.size) },
        {
            label: "アップストリーム",
            value: pkg.url ? (
                <a
                    href={pkg.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-link hover:underline"
                >
                    {pkg.url}
                    <ExternalLink className="h-3.5 w-3.5" />
                </a>
            ) : (
                "—"
            ),
        },
        { label: "種別", value: pkg.pkgtype || "—" },
    ];

    return (
        <PageContainer
            measure="full"
            header={
                <PageHeader
                    title={pkg.pkgname}
                    description={pkg.pkgdesc || undefined}
                    breadcrumbs={[packagesCrumb, { label: pkg.pkgname }]}
                    actions={
                        <>
                            <Button
                                onClick={handleDownload}
                                className="h-9 gap-1.5 rounded-sm text-[14px]"
                            >
                                <Download className="h-4 w-4" />
                                ダウンロード
                            </Button>
                            <BugReportDialog
                                packageInfo={pkg}
                                trigger={
                                    <Button
                                        variant="outline"
                                        className="h-9 gap-1.5 rounded-sm text-[14px]"
                                    >
                                        <BugIcon className="h-4 w-4" />
                                        バグ報告
                                    </Button>
                                }
                            />
                        </>
                    }
                />
            }
        >
            <div className="space-y-8">
                <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                    <span className="font-mono text-[15px] text-muted-foreground">
                        {pkg.pkgver}
                    </span>
                    <span className="text-[14px] text-muted-foreground">
                        ({pkg.arch})
                    </span>
                </div>

                <div className="overflow-hidden rounded-sm border border-border">
                    <table className="w-full text-[15px]">
                        <tbody>
                            {meta.map((row, i) => (
                                <tr
                                    key={row.label}
                                    className={
                                        i % 2 === 1
                                            ? "bg-table-stripe"
                                            : "bg-card"
                                    }
                                >
                                    <th className="w-44 whitespace-nowrap border-b border-border px-4 py-2.5 text-left align-top font-semibold text-muted-foreground">
                                        {row.label}
                                    </th>
                                    <td className="break-all border-b border-border px-4 py-2.5 align-top">
                                        {row.value}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>

                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                    <PkgDepSection
                        title="依存関係"
                        items={pkg.depend}
                        repo={repo}
                        arch={arch}
                        link
                    />
                    <PkgDepSection
                        title="オプション依存"
                        items={pkg.optdepend}
                    />
                    <PkgDepSection
                        title="ビルド依存"
                        items={pkg.makedepend}
                        repo={repo}
                        arch={arch}
                        link
                    />
                    <PkgDepSection
                        title="チェック依存"
                        items={pkg.checkdepend}
                        repo={repo}
                        arch={arch}
                        link
                    />
                    <PkgDepSection title="提供" items={pkg.provides} />
                    <PkgDepSection title="競合" items={pkg.conflict} />
                    <PkgDepSection title="置換" items={pkg.replaces} />
                </div>
            </div>
        </PageContainer>
    );
}
