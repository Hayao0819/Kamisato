"use client";

import { AlertCircle, Ban, Hammer, ScrollText } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { useAPIClient } from "@/components/lumine-provider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";
import type { BuildRequest, BuildStats, Job, JobStatus } from "@/lib/types";

const POLL_INTERVAL_MS = 3000;

const statusVariant: Record<
    JobStatus,
    "default" | "secondary" | "destructive" | "outline"
> = {
    queued: "secondary",
    running: "default",
    success: "outline",
    failed: "destructive",
    cancelled: "outline",
};

const statusLabel: Record<JobStatus, string> = {
    queued: "待機中",
    running: "実行中",
    success: "成功",
    failed: "失敗",
    cancelled: "キャンセル",
};

const ARCH_OPTIONS = ["x86_64", "aarch64", "armv7h"];

type SourceKind = "pkgbuild" | "git";

export function BuildsPageClient() {
    const api = useAPIClient();
    const { toast } = useToast();
    const { isAuthenticated, username, password } = useAuth();

    const [jobs, setJobs] = useState<Job[]>([]);
    const [stats, setStats] = useState<BuildStats | null>(null);
    const [submitting, setSubmitting] = useState(false);

    const [repo, setRepo] = useState("");
    const [arch, setArch] = useState("x86_64");
    const [sourceKind, setSourceKind] = useState<SourceKind>("pkgbuild");
    const [pkgbuild, setPkgbuild] = useState("");
    const [gitUrl, setGitUrl] = useState("");
    const [gitRef, setGitRef] = useState("");
    const [gpgKey, setGpgKey] = useState("");
    const [timeout, setTimeout] = useState("");

    const [logJobId, setLogJobId] = useState<string | null>(null);
    const [logLines, setLogLines] = useState<string[]>([]);

    const executable = api.endpoints.executable;

    const refreshJobs = useCallback(async () => {
        try {
            const data = await api.listJobs();
            setJobs(Array.isArray(data) ? data : []);
        } catch (error) {
            console.error("Failed to fetch jobs", error);
        }
    }, [api]);

    const refreshStats = useCallback(async () => {
        try {
            setStats(await api.fetchStats());
        } catch (error) {
            console.error("Failed to fetch stats", error);
        }
    }, [api]);

    useEffect(() => {
        if (!executable) return;
        refreshJobs();
        refreshStats();
        const timer = setInterval(() => {
            refreshJobs();
            refreshStats();
        }, POLL_INTERVAL_MS);
        return () => clearInterval(timer);
    }, [executable, refreshJobs, refreshStats]);

    // EventSource streams live logs; reset and reconnect when the target changes.
    useEffect(() => {
        if (!logJobId) return;
        setLogLines([]);
        const source = new EventSource(api.jobLogsUrl(logJobId));
        source.onmessage = (e) => {
            setLogLines((prev) => [...prev, e.data]);
        };
        source.onerror = () => {
            source.close();
        };
        return () => source.close();
    }, [logJobId, api]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!repo.trim() || !arch.trim()) {
            toast({
                title: "エラー",
                description: "リポジトリとアーキテクチャを入力してください",
                variant: "destructive",
            });
            return;
        }

        const req: BuildRequest = { repo: repo.trim(), arch: arch.trim() };
        if (sourceKind === "git") {
            if (!gitUrl.trim()) {
                toast({
                    title: "エラー",
                    description: "git URL を入力してください",
                    variant: "destructive",
                });
                return;
            }
            req.git = { url: gitUrl.trim() };
            if (gitRef.trim()) req.git.ref = gitRef.trim();
        } else {
            if (!pkgbuild.trim()) {
                toast({
                    title: "エラー",
                    description: "PKGBUILD を入力してください",
                    variant: "destructive",
                });
                return;
            }
            req.pkgbuild = pkgbuild;
        }
        if (gpgKey.trim()) req.gpg_key = gpgKey.trim();
        const timeoutMin = Number.parseInt(timeout, 10);
        if (Number.isFinite(timeoutMin) && timeoutMin > 0)
            req.timeout = timeoutMin;

        setSubmitting(true);
        try {
            const { job_id } = await api.submitBuild(
                req,
                username || undefined,
                password || undefined,
            );
            toast({
                title: "投入しました",
                description: `ジョブ ${job_id} を投入しました`,
            });
            setLogJobId(job_id);
            refreshJobs();
        } catch (error) {
            toast({
                title: "エラー",
                description:
                    error instanceof Error
                        ? error.message
                        : "ビルドの投入に失敗しました",
                variant: "destructive",
            });
        } finally {
            setSubmitting(false);
        }
    };

    const handleCancel = async (id: string) => {
        try {
            await api.cancelJob(
                id,
                username || undefined,
                password || undefined,
            );
            toast({
                title: "キャンセルしました",
                description: `ジョブ ${id} をキャンセルしました`,
            });
            refreshJobs();
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
    };

    return (
        <div className="container mx-auto py-8 px-4 max-w-4xl space-y-6">
            {!isAuthenticated && (
                <Alert>
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>認証が必要な場合があります</AlertTitle>
                    <AlertDescription>
                        サーバー設定により、ビルドの投入に認証が必要な場合があります。
                        ヘッダーのログインボタンから認証情報を設定してください。
                    </AlertDescription>
                </Alert>
            )}

            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Hammer className="w-6 h-6" />
                        ビルドの投入
                    </CardTitle>
                    <CardDescription>
                        ビルドサーバーにパッケージのビルドジョブを投入します
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <form onSubmit={handleSubmit} className="space-y-6">
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="repo">リポジトリ</Label>
                                <Input
                                    id="repo"
                                    value={repo}
                                    onChange={(e) => setRepo(e.target.value)}
                                    placeholder="例: extra"
                                    required
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="arch">アーキテクチャ</Label>
                                <Select value={arch} onValueChange={setArch}>
                                    <SelectTrigger id="arch">
                                        <SelectValue placeholder="アーキテクチャを選択" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {ARCH_OPTIONS.map((a) => (
                                            <SelectItem key={a} value={a}>
                                                {a}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>

                        <div className="space-y-2">
                            <Label>ビルドソース</Label>
                            <div className="flex gap-2">
                                <Button
                                    type="button"
                                    variant={
                                        sourceKind === "pkgbuild"
                                            ? "default"
                                            : "outline"
                                    }
                                    onClick={() => setSourceKind("pkgbuild")}
                                >
                                    PKGBUILD
                                </Button>
                                <Button
                                    type="button"
                                    variant={
                                        sourceKind === "git"
                                            ? "default"
                                            : "outline"
                                    }
                                    onClick={() => setSourceKind("git")}
                                >
                                    git
                                </Button>
                            </div>
                        </div>

                        {sourceKind === "pkgbuild" ? (
                            <div className="space-y-2">
                                <Label htmlFor="pkgbuild">PKGBUILD</Label>
                                <Textarea
                                    id="pkgbuild"
                                    value={pkgbuild}
                                    onChange={(e) =>
                                        setPkgbuild(e.target.value)
                                    }
                                    placeholder="pkgname=..."
                                    className="min-h-[200px] font-mono text-sm"
                                />
                            </div>
                        ) : (
                            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                                <div className="space-y-2">
                                    <Label htmlFor="git-url">git URL</Label>
                                    <Input
                                        id="git-url"
                                        value={gitUrl}
                                        onChange={(e) =>
                                            setGitUrl(e.target.value)
                                        }
                                        placeholder="https://aur.archlinux.org/foo.git"
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="git-ref">
                                        ref (オプション)
                                    </Label>
                                    <Input
                                        id="git-ref"
                                        value={gitRef}
                                        onChange={(e) =>
                                            setGitRef(e.target.value)
                                        }
                                        placeholder="例: master"
                                    />
                                </div>
                            </div>
                        )}

                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="gpg-key">
                                    GPG 鍵 (オプション)
                                </Label>
                                <Input
                                    id="gpg-key"
                                    value={gpgKey}
                                    onChange={(e) => setGpgKey(e.target.value)}
                                    placeholder="署名に使う鍵 ID"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="timeout">
                                    タイムアウト(分) (オプション)
                                </Label>
                                <Input
                                    id="timeout"
                                    type="number"
                                    min={0}
                                    value={timeout}
                                    onChange={(e) => setTimeout(e.target.value)}
                                    placeholder="0 でサーバー既定"
                                />
                            </div>
                        </div>

                        <Button
                            type="submit"
                            className="w-full"
                            disabled={submitting || !executable}
                        >
                            {submitting ? (
                                <>
                                    <span className="animate-spin mr-2">
                                        ⏳
                                    </span>
                                    投入中...
                                </>
                            ) : (
                                <>
                                    <Hammer className="w-4 h-4 mr-2" />
                                    ビルドを投入
                                </>
                            )}
                        </Button>
                    </form>
                </CardContent>
            </Card>

            {stats && (
                <Card>
                    <CardHeader>
                        <CardTitle className="text-base">
                            ビルドサーバーの状態
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="grid grid-cols-2 sm:grid-cols-5 gap-4 text-sm">
                            <div className="space-y-1">
                                <p className="text-muted-foreground">
                                    ワーカー
                                </p>
                                <p className="text-lg font-semibold">
                                    {stats.workers}
                                </p>
                            </div>
                            <div className="space-y-1">
                                <p className="text-muted-foreground">
                                    キュー待ち
                                </p>
                                <p className="text-lg font-semibold">
                                    {stats.queue_length}
                                </p>
                            </div>
                            <div className="space-y-1">
                                <p className="text-muted-foreground">実行中</p>
                                <p className="text-lg font-semibold">
                                    {stats.running}
                                </p>
                            </div>
                            <div className="space-y-1">
                                <p className="text-muted-foreground">合計</p>
                                <p className="text-lg font-semibold">
                                    {stats.total}
                                </p>
                            </div>
                            <div className="space-y-1">
                                <p className="text-muted-foreground">成功率</p>
                                <p className="text-lg font-semibold">
                                    {Math.round(stats.success_rate * 100)}%
                                </p>
                            </div>
                        </div>
                    </CardContent>
                </Card>
            )}

            <Card>
                <CardHeader>
                    <CardTitle>ジョブ一覧</CardTitle>
                    <CardDescription>
                        数秒ごとに自動更新されます
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    {jobs.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                            ジョブはありません
                        </p>
                    ) : (
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>ID</TableHead>
                                    <TableHead>リポジトリ</TableHead>
                                    <TableHead>Arch</TableHead>
                                    <TableHead>状態</TableHead>
                                    <TableHead className="text-right">
                                        ログ
                                    </TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {jobs.map((job) => (
                                    <TableRow key={job.id}>
                                        <TableCell className="font-mono text-xs">
                                            {job.id}
                                        </TableCell>
                                        <TableCell>{job.repo}</TableCell>
                                        <TableCell>{job.arch}</TableCell>
                                        <TableCell>
                                            <Badge
                                                variant={
                                                    statusVariant[job.status]
                                                }
                                            >
                                                {statusLabel[job.status] ??
                                                    job.status}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="text-right">
                                            <div className="flex justify-end gap-1">
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() =>
                                                        setLogJobId(job.id)
                                                    }
                                                >
                                                    <ScrollText className="w-4 h-4" />
                                                </Button>
                                                {(job.status === "queued" ||
                                                    job.status ===
                                                        "running") && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        onClick={() =>
                                                            handleCancel(job.id)
                                                        }
                                                        title="キャンセル"
                                                    >
                                                        <Ban className="w-4 h-4" />
                                                    </Button>
                                                )}
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </CardContent>
            </Card>

            {logJobId && (
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2 text-base">
                            <ScrollText className="w-5 h-5" />
                            ログ: {logJobId}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <pre className="max-h-96 overflow-auto rounded-md bg-muted p-4 text-xs whitespace-pre-wrap">
                            {logLines.length > 0
                                ? logLines.join("\n")
                                : "ログを待機しています..."}
                        </pre>
                    </CardContent>
                </Card>
            )}
        </div>
    );
}
