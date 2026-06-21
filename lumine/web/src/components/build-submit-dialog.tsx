"use client";

import { Hammer, Loader2 } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import type { BuildRequest } from "@/lib/types";

const ARCH_OPTIONS = ["x86_64", "aarch64", "armv7h"];

type SourceKind = "pkgbuild" | "git";

export function BuildSubmitDialog({
    disabled,
    onSubmit,
}: {
    disabled?: boolean;
    onSubmit: (req: BuildRequest) => Promise<boolean>;
}) {
    const [open, setOpen] = useState(false);
    const [submitting, setSubmitting] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const [repo, setRepo] = useState("");
    const [arch, setArch] = useState("x86_64");
    const [sourceKind, setSourceKind] = useState<SourceKind>("pkgbuild");
    const [pkgbuild, setPkgbuild] = useState("");
    const [gitUrl, setGitUrl] = useState("");
    const [gitRef, setGitRef] = useState("");
    const [installPkgs, setInstallPkgs] = useState("");
    const [gpgKey, setGpgKey] = useState("");
    const [timeout, setTimeout] = useState("");

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError(null);

        if (!repo.trim()) {
            setError("リポジトリを入力してください");
            return;
        }
        const req: BuildRequest = { repo: repo.trim(), arch: arch.trim() };
        if (sourceKind === "git") {
            if (!gitUrl.trim()) {
                setError("git URL を入力してください");
                return;
            }
            req.git = { url: gitUrl.trim() };
            if (gitRef.trim()) req.git.ref = gitRef.trim();
        } else {
            if (!pkgbuild.trim()) {
                setError("PKGBUILD を入力してください");
                return;
            }
            req.pkgbuild = pkgbuild;
        }
        const pkgs = installPkgs
            .split(/[\s,]+/)
            .map((s) => s.trim())
            .filter(Boolean);
        if (pkgs.length > 0) req.install_pkgs = pkgs;
        if (gpgKey.trim()) req.gpg_key = gpgKey.trim();
        const timeoutMin = Number.parseInt(timeout, 10);
        if (Number.isFinite(timeoutMin) && timeoutMin > 0)
            req.timeout = timeoutMin;

        setSubmitting(true);
        try {
            const ok = await onSubmit(req);
            if (ok) {
                setOpen(false);
                setPkgbuild("");
                setGitUrl("");
                setGitRef("");
                setInstallPkgs("");
                setGpgKey("");
                setTimeout("");
            }
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                <Button size="sm" disabled={disabled}>
                    <Hammer className="mr-2 h-4 w-4" />
                    ビルドを投入
                </Button>
            </DialogTrigger>
            <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle>ビルドを投入</DialogTitle>
                    <DialogDescription>
                        ビルドサーバーにパッケージのビルドジョブを投入します
                    </DialogDescription>
                </DialogHeader>

                <form onSubmit={handleSubmit} className="space-y-5">
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                        <div className="space-y-2">
                            <Label htmlFor="build-repo">リポジトリ</Label>
                            <Input
                                id="build-repo"
                                value={repo}
                                onChange={(e) => setRepo(e.target.value)}
                                placeholder="例: extra"
                                required
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="build-arch">アーキテクチャ</Label>
                            <Select value={arch} onValueChange={setArch}>
                                <SelectTrigger id="build-arch">
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
                        <div className="inline-flex rounded-sm border border-border p-0.5">
                            {(["pkgbuild", "git"] as SourceKind[]).map((k) => (
                                <button
                                    key={k}
                                    type="button"
                                    onClick={() => setSourceKind(k)}
                                    className={cn(
                                        "rounded-sm px-3 py-1 text-[13px] transition-colors",
                                        sourceKind === k
                                            ? "bg-primary text-primary-foreground"
                                            : "text-muted-foreground hover:text-foreground",
                                    )}
                                >
                                    {k === "pkgbuild" ? "PKGBUILD" : "git"}
                                </button>
                            ))}
                        </div>
                    </div>

                    {sourceKind === "pkgbuild" ? (
                        <div className="space-y-2">
                            <Label htmlFor="build-pkgbuild">PKGBUILD</Label>
                            <Textarea
                                id="build-pkgbuild"
                                value={pkgbuild}
                                onChange={(e) => setPkgbuild(e.target.value)}
                                placeholder="pkgname=..."
                                className="min-h-[180px] font-mono text-sm"
                            />
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                            <div className="space-y-2">
                                <Label htmlFor="build-git-url">git URL</Label>
                                <Input
                                    id="build-git-url"
                                    value={gitUrl}
                                    onChange={(e) => setGitUrl(e.target.value)}
                                    placeholder="https://aur.archlinux.org/foo.git"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="build-git-ref">
                                    ref (任意)
                                </Label>
                                <Input
                                    id="build-git-ref"
                                    value={gitRef}
                                    onChange={(e) => setGitRef(e.target.value)}
                                    placeholder="例: master"
                                />
                            </div>
                        </div>
                    )}

                    <div className="space-y-2">
                        <Label htmlFor="build-install-pkgs">
                            追加インストールパッケージ (任意)
                        </Label>
                        <Input
                            id="build-install-pkgs"
                            value={installPkgs}
                            onChange={(e) => setInstallPkgs(e.target.value)}
                            placeholder="空白かカンマ区切り 例: git base-devel"
                        />
                    </div>

                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                        <div className="space-y-2">
                            <Label htmlFor="build-gpg-key">
                                GPG 鍵 (任意)
                            </Label>
                            <Input
                                id="build-gpg-key"
                                value={gpgKey}
                                onChange={(e) => setGpgKey(e.target.value)}
                                placeholder="署名に使う鍵 ID"
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="build-timeout">
                                タイムアウト(分) (任意)
                            </Label>
                            <Input
                                id="build-timeout"
                                type="number"
                                min={0}
                                value={timeout}
                                onChange={(e) => setTimeout(e.target.value)}
                                placeholder="0 でサーバー既定"
                            />
                        </div>
                    </div>

                    {error && (
                        <p className="text-sm text-destructive">{error}</p>
                    )}

                    <div className="flex justify-end gap-2 pt-1">
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => setOpen(false)}
                            disabled={submitting}
                        >
                            キャンセル
                        </Button>
                        <Button type="submit" disabled={submitting || disabled}>
                            {submitting ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    投入中...
                                </>
                            ) : (
                                <>
                                    <Hammer className="mr-2 h-4 w-4" />
                                    投入する
                                </>
                            )}
                        </Button>
                    </div>
                </form>
            </DialogContent>
        </Dialog>
    );
}
