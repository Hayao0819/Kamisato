"use client";

import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useRepoArch } from "@/hooks/use-repo-arch";

export function RepoArchSelector() {
    const {
        selectedRepo,
        setSelectedRepo,
        selectedArch,
        setSelectedArch,
        repos,
        arches,
    } = useRepoArch();

    return (
        <div className="flex flex-wrap items-end gap-4">
            <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-muted-foreground">
                    リポジトリ
                </span>
                <Select
                    value={selectedRepo || undefined}
                    onValueChange={setSelectedRepo}
                    disabled={repos.length === 0}
                >
                    <SelectTrigger className="w-[180px] h-8 rounded-sm">
                        <SelectValue placeholder="リポジトリを選択" />
                    </SelectTrigger>
                    <SelectContent>
                        {repos.map((repo) => (
                            <SelectItem key={repo} value={repo}>
                                {repo}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </label>

            <label className="flex flex-col gap-1">
                <span className="text-xs font-medium text-muted-foreground">
                    アーキテクチャ
                </span>
                <Select
                    value={selectedArch || undefined}
                    onValueChange={setSelectedArch}
                    disabled={arches.length === 0}
                >
                    <SelectTrigger className="w-[180px] h-8 rounded-sm">
                        <SelectValue placeholder="アーキテクチャを選択" />
                    </SelectTrigger>
                    <SelectContent>
                        {arches.map((arch) => (
                            <SelectItem key={arch} value={arch}>
                                {arch}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </label>
        </div>
    );
}
