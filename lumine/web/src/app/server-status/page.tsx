"use client";

import {
    Activity,
    CheckCircle2,
    Clock,
    Cpu,
    ListChecks,
    Loader2,
    Server,
    XCircle,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { STATUS_LABEL, STATUS_ORDER } from "@/components/build-status";
import { ScopeBar } from "@/components/console-scope-bar";
import { useAPIClient } from "@/components/lumine-provider";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { StatusRefreshButton } from "@/components/status-refresh-button";
import { StatusTile } from "@/components/status-tile";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import type { BuildStats, JobStatus } from "@/lib/types";

type Health = "online" | "offline" | "loading";

function formatUptime(sec: number): string {
    if (!Number.isFinite(sec) || sec <= 0) return "0秒";
    const s = Math.floor(sec);
    const days = Math.floor(s / 86400);
    const hours = Math.floor((s % 86400) / 3600);
    const mins = Math.floor((s % 3600) / 60);
    const parts: string[] = [];
    if (days) parts.push(`${days}日`);
    if (hours) parts.push(`${hours}時間`);
    if (mins || parts.length === 0) parts.push(`${mins}分`);
    return parts.join("");
}

export default function ServerStatusPage() {
    const api = useAPIClient();
    const apiRef = useRef(api);
    useEffect(() => {
        apiRef.current = api;
    }, [api]);

    const [stats, setStats] = useState<BuildStats | null>(null);
    const [health, setHealth] = useState<Health>("loading");
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);

    const refresh = useCallback(async () => {
        setLoading(true);
        const healthP = apiRef.current
            .fetchHello()
            .then(
                (res): Health =>
                    res.ok || res.status === 418 ? "online" : "offline",
            )
            .catch((): Health => "offline");
        const statsP = apiRef.current
            .fetchStats()
            .then((s) => ({ ok: true as const, s }))
            .catch((e) => ({
                ok: false as const,
                e: e instanceof Error ? e.message : String(e),
            }));

        const [h, statsRes] = await Promise.all([healthP, statsP]);
        setHealth(h);
        if (statsRes.ok) {
            setStats(statsRes.s);
            setError(null);
        } else {
            setError(statsRes.e);
        }
        setLoading(false);
    }, []);

    useEffect(() => {
        if (!api.endpoints.executable) return;
        refresh();
        const timer = setInterval(refresh, 10000);
        return () => clearInterval(timer);
    }, [api.endpoints.executable, refresh]);

    const successRate = stats
        ? `${(stats.success_rate * 100).toFixed(1)}%`
        : "—";

    return (
        <>
            <ScopeBar />
            <PageContainer
                measure="full"
                header={
                    <PageHeader
                        title="サーバー状態"
                        description="ビルドサーバー (miko) の稼働状況とジョブ統計"
                        actions={
                            <StatusRefreshButton
                                onRefresh={refresh}
                                loading={loading}
                            />
                        }
                    />
                }
            >
                <div className="space-y-6">
                    {error && (
                        <Alert variant="destructive">
                            <XCircle className="h-4 w-4" />
                            <AlertTitle>統計の取得に失敗しました</AlertTitle>
                            <AlertDescription>{error}</AlertDescription>
                        </Alert>
                    )}

                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
                        <StatusTile
                            label="ワーカー数"
                            value={stats?.workers ?? "—"}
                            icon={<Cpu className="h-4 w-4" />}
                        />
                        <StatusTile
                            label="キュー長"
                            value={stats?.queue_length ?? "—"}
                            icon={<ListChecks className="h-4 w-4" />}
                        />
                        <StatusTile
                            label="実行中"
                            value={stats?.running ?? "—"}
                            icon={<Loader2 className="h-4 w-4" />}
                        />
                        <StatusTile
                            label="成功率"
                            value={successRate}
                            icon={<CheckCircle2 className="h-4 w-4" />}
                        />
                        <StatusTile
                            label="総ジョブ"
                            value={stats?.total ?? "—"}
                            icon={<Activity className="h-4 w-4" />}
                        />
                        <StatusTile
                            label="稼働時間"
                            value={stats ? formatUptime(stats.uptime_sec) : "—"}
                            icon={<Clock className="h-4 w-4" />}
                        />
                    </div>

                    <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
                        <div className="rounded-md border bg-card p-4 text-card-foreground lg:col-span-2">
                            <div className="mb-3 flex items-center gap-2">
                                <span className="text-sm font-medium">
                                    ステータス別ジョブ数
                                </span>
                            </div>
                            <StatusBreakdown counts={stats?.counts} />
                        </div>

                        <div className="rounded-md border bg-card p-4 text-card-foreground">
                            <div className="mb-3 flex items-center gap-2 text-sm font-medium">
                                <Server className="h-4 w-4 text-muted-foreground" />
                                ヘルス
                            </div>
                            <HealthIndicator health={health} />
                        </div>
                    </div>
                </div>
            </PageContainer>
        </>
    );
}

function StatusBreakdown({ counts }: { counts?: Record<string, number> }) {
    const total =
        STATUS_ORDER.reduce((acc, s) => acc + (counts?.[s] ?? 0), 0) || 0;
    return (
        <div className="space-y-3">
            <div className="flex flex-wrap gap-2">
                {STATUS_ORDER.map((s) => (
                    <Badge key={s} variant="outline" className="gap-1.5">
                        {STATUS_LABEL[s as JobStatus]}
                        <span className="font-semibold tabular-nums">
                            {counts?.[s] ?? 0}
                        </span>
                    </Badge>
                ))}
            </div>
            <div className="flex h-2.5 w-full overflow-hidden rounded-full bg-muted">
                {total > 0 &&
                    STATUS_ORDER.map((s) => {
                        const n = counts?.[s] ?? 0;
                        if (n === 0) return null;
                        return (
                            <div
                                key={s}
                                className={BAR_COLOR[s as JobStatus]}
                                style={{ width: `${(n / total) * 100}%` }}
                                title={`${STATUS_LABEL[s as JobStatus]}: ${n}`}
                            />
                        );
                    })}
            </div>
        </div>
    );
}

const BAR_COLOR: Record<JobStatus, string> = {
    queued: "bg-muted-foreground/40",
    running: "bg-primary",
    success: "bg-emerald-500",
    failed: "bg-destructive",
    cancelled: "bg-muted-foreground/60",
};

function HealthIndicator({ health }: { health: Health }) {
    if (health === "loading") {
        return (
            <div className="flex items-center gap-2 text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span className="text-sm">確認中...</span>
            </div>
        );
    }
    const online = health === "online";
    return (
        <div className="flex items-center gap-2">
            <span
                className={`flex h-2.5 w-2.5 rounded-full ${
                    online ? "bg-emerald-500" : "bg-destructive"
                }`}
            />
            <span className="text-lg font-semibold">
                {online ? "オンライン" : "オフライン"}
            </span>
        </div>
    );
}
