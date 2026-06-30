"use client";

import { MoreHorizontal } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AuthGate } from "@/components/auth-gate";
import { BuildJobDetail } from "@/components/build-job-detail";
import { BuildQueueIndicator } from "@/components/build-queue-indicator";
import {
    formatDuration,
    STATUS_LABEL,
    STATUS_VARIANT,
} from "@/components/build-status";
import {
    BuildStatusFilter,
    type StatusFilter,
} from "@/components/build-status-filter";
import { BuildSubmitDialog } from "@/components/build-submit-dialog";
import { useAPIClient, useFeatures } from "@/components/lumine-provider";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table";
import { useToast } from "@/hooks/use-toast";
import type { BuildRequest, BuildStats, Job, JobStatus } from "@/lib/types";

const POLL_INTERVAL_MS = 3000;

function emptyCounts(): Record<JobStatus, number> {
    return {
        queued: 0,
        running: 0,
        success: 0,
        failed: 0,
        cancelled: 0,
    };
}

function pkgLabel(job: Job): string {
    const pkgs = job.packages ?? [];
    if (pkgs.length === 0) return "—";
    const name =
        pkgs[0]
            .split("/")
            .pop()
            ?.replace(/\.pkg\.tar\.[a-z0-9]+$/i, "") ?? pkgs[0];
    return pkgs.length > 1 ? `${name} 他${pkgs.length - 1}件` : name;
}

export function BuildsPageClient() {
    const api = useAPIClient();
    const features = useFeatures();
    const { toast } = useToast();

    const [jobs, setJobs] = useState<Job[]>([]);
    const [stats, setStats] = useState<BuildStats | null>(null);
    const [filter, setFilter] = useState<StatusFilter>("all");
    const [detailId, setDetailId] = useState<string | null>(null);
    const [highlightId, setHighlightId] = useState<string | null>(null);
    const searchParams = useSearchParams();

    const executable = api.endpoints.executable;

    const refresh = useCallback(async () => {
        try {
            const data = await api.listJobs();
            setJobs(Array.isArray(data) ? data : []);
        } catch (error) {
            console.error("Failed to fetch jobs", error);
        }
        try {
            setStats(await api.fetchStats());
        } catch (error) {
            console.error("Failed to fetch stats", error);
        }
    }, [api]);

    useEffect(() => {
        if (!executable || !features.miko) return;
        refresh();
        const timer = setInterval(refresh, POLL_INTERVAL_MS);
        return () => clearInterval(timer);
    }, [executable, features.miko, refresh]);

    const sorted = useMemo(() => {
        return [...jobs].sort((a, b) => {
            const ta = a.created_at ? Date.parse(a.created_at) : 0;
            const tb = b.created_at ? Date.parse(b.created_at) : 0;
            return tb - ta;
        });
    }, [jobs]);

    const counts = useMemo(() => {
        const c = emptyCounts();
        for (const job of jobs) {
            if (job.status in c) c[job.status] += 1;
        }
        return c;
    }, [jobs]);

    const visible = useMemo(
        () =>
            filter === "all"
                ? sorted
                : sorted.filter((j) => j.status === filter),
        [sorted, filter],
    );

    const detailJob = useMemo(
        () => jobs.find((j) => j.id === detailId) ?? null,
        [jobs, detailId],
    );

    const handleSubmit = useCallback(
        async (req: BuildRequest): Promise<boolean> => {
            try {
                const { job_id } = await api.submitBuild(req);
                toast({
                    title: "投入しました",
                    description: `ジョブ ${job_id.slice(0, 8)} を投入しました`,
                });
                setHighlightId(job_id);
                setFilter("all");
                await refresh();
                return true;
            } catch (error) {
                toast({
                    title: "エラー",
                    description:
                        error instanceof Error
                            ? error.message
                            : "ビルドの投入に失敗しました",
                    variant: "destructive",
                });
                return false;
            }
        },
        [api, toast, refresh],
    );

    const handleCancel = useCallback(
        async (id: string) => {
            try {
                await api.cancelJob(id);
                toast({
                    title: "キャンセルしました",
                    description: `ジョブ ${id.slice(0, 8)} をキャンセルしました`,
                });
                await refresh();
            } catch (error) {
                toast({
                    title: "エラー",
                    description:
                        error instanceof Error
                            ? error.message
                            : "ジョブのキャンセルに失敗しました",
                    variant: "destructive",
                });
            }
        },
        [api, toast, refresh],
    );

    // Drop the highlight once the row is no longer the freshest focus.
    useEffect(() => {
        if (!highlightId) return;
        const t = window.setTimeout(() => setHighlightId(null), 6000);
        return () => window.clearTimeout(t);
    }, [highlightId]);

    if (!features.miko) {
        return (
            <PageContainer
                measure="full"
                header={<PageHeader title="ビルド" />}
            >
                <p className="text-sm text-muted-foreground">
                    ビルド機能は現在利用できません。
                </p>
            </PageContainer>
        );
    }

    return (
        <PageContainer
            measure="full"
            header={
                <PageHeader
                    title="ビルド"
                    description="ビルドジョブの状態とログ"
                    actions={
                        <AuthGate fallback={null}>
                            <BuildSubmitDialog
                                disabled={!executable}
                                onSubmit={handleSubmit}
                                initialRepo={
                                    searchParams.get("repo") ?? undefined
                                }
                                initialArch={
                                    searchParams.get("arch") ?? undefined
                                }
                                defaultOpen={searchParams.has("repo")}
                            />
                        </AuthGate>
                    }
                />
            }
        >
            <div className="space-y-5">
                <BuildQueueIndicator stats={stats} />

                <BuildStatusFilter
                    value={filter}
                    counts={counts}
                    total={jobs.length}
                    onChange={setFilter}
                />

                <div className="overflow-hidden rounded-md border border-border">
                    {visible.length === 0 ? (
                        <p className="px-4 py-10 text-center text-sm text-muted-foreground">
                            {jobs.length === 0
                                ? "ジョブはありません"
                                : "該当するジョブはありません"}
                        </p>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>パッケージ</TableHead>
                                    <TableHead>リポ/アーキ</TableHead>
                                    <TableHead>状態</TableHead>
                                    <TableHead>作成日時</TableHead>
                                    <TableHead>所要時間</TableHead>
                                    <TableHead className="w-10" />
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {visible.map((job) => (
                                    <TableRow
                                        key={job.id}
                                        onClick={() => setDetailId(job.id)}
                                        data-highlight={
                                            job.id === highlightId || undefined
                                        }
                                        className="cursor-pointer data-[highlight]:bg-primary/10"
                                    >
                                        <TableCell className="max-w-[280px] truncate font-mono text-[13px]">
                                            {job.packages?.length ? (
                                                pkgLabel(job)
                                            ) : (
                                                <span className="text-muted-foreground">
                                                    —
                                                </span>
                                            )}
                                        </TableCell>
                                        <TableCell className="whitespace-nowrap">
                                            <span className="text-foreground">
                                                {job.repo}
                                            </span>
                                            <span className="text-muted-foreground/60">
                                                {" / "}
                                            </span>
                                            <span className="text-muted-foreground">
                                                {job.arch}
                                            </span>
                                        </TableCell>
                                        <TableCell>
                                            <Badge
                                                variant={
                                                    STATUS_VARIANT[job.status]
                                                }
                                            >
                                                {STATUS_LABEL[job.status] ??
                                                    job.status}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="whitespace-nowrap tabular-nums text-muted-foreground">
                                            {job.created_at
                                                ? new Date(
                                                      job.created_at,
                                                  ).toLocaleString("ja-JP")
                                                : "—"}
                                        </TableCell>
                                        <TableCell className="whitespace-nowrap tabular-nums text-muted-foreground">
                                            {formatDuration(
                                                job.started_at,
                                                job.ended_at,
                                            )}
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="h-7 w-7 text-muted-foreground"
                                                aria-label="詳細"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    setDetailId(job.id);
                                                }}
                                            >
                                                <MoreHorizontal className="h-4 w-4" />
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </div>
            </div>

            <BuildJobDetail
                job={detailJob}
                open={detailId !== null}
                onOpenChange={(o) => {
                    if (!o) setDetailId(null);
                }}
                onCancel={handleCancel}
            />
        </PageContainer>
    );
}
