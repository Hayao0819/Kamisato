"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AuthGate } from "@/components/auth-gate";
import { BuildJobDetail } from "@/components/build-job-detail";
import { BuildQueueIndicator } from "@/components/build-queue-indicator";
import {
    BuildStatusFilter,
    type StatusFilter,
} from "@/components/build-status-filter";
import { BuildSubmitDialog } from "@/components/build-submit-dialog";
import {
    STATUS_LABEL,
    STATUS_VARIANT,
    formatDuration,
} from "@/components/build-status";
import { useAuth } from "@/components/auth-provider";
import { useAPIClient } from "@/components/lumine-provider";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
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

export function BuildsPageClient() {
    const api = useAPIClient();
    const { toast } = useToast();
    const { username, password } = useAuth();

    const [jobs, setJobs] = useState<Job[]>([]);
    const [stats, setStats] = useState<BuildStats | null>(null);
    const [filter, setFilter] = useState<StatusFilter>("all");
    const [detailId, setDetailId] = useState<string | null>(null);
    const [highlightId, setHighlightId] = useState<string | null>(null);

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
        if (!executable) return;
        refresh();
        const timer = setInterval(refresh, POLL_INTERVAL_MS);
        return () => clearInterval(timer);
    }, [executable, refresh]);

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
                const { job_id } = await api.submitBuild(
                    req,
                    username || undefined,
                    password || undefined,
                );
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
        [api, username, password, toast, refresh],
    );

    const handleCancel = useCallback(
        async (id: string) => {
            try {
                await api.cancelJob(
                    id,
                    username || undefined,
                    password || undefined,
                );
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
        [api, username, password, toast, refresh],
    );

    // Drop the highlight once the row is no longer the freshest focus.
    useEffect(() => {
        if (!highlightId) return;
        const t = window.setTimeout(() => setHighlightId(null), 6000);
        return () => window.clearTimeout(t);
    }, [highlightId]);

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
                                    <TableHead>ID</TableHead>
                                    <TableHead>リポ/アーキ</TableHead>
                                    <TableHead>状態</TableHead>
                                    <TableHead>作成日時</TableHead>
                                    <TableHead>所要時間</TableHead>
                                    <TableHead className="text-right">
                                        パッケージ
                                    </TableHead>
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
                                        <TableCell className="font-mono text-xs">
                                            {job.id.slice(0, 8)}
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
                                        <TableCell className="text-right tabular-nums">
                                            {job.packages?.length ?? 0}
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
