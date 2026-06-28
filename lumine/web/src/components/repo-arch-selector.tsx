"use client";

import { Label } from "@/components/ui/label";
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
            <div className="flex flex-col gap-1">
                <Label
                    htmlFor="repo-select"
                    className="text-xs font-medium text-muted-foreground"
                >
                    リポジトリ
                </Label>
                <Select
                    value={selectedRepo || undefined}
                    onValueChange={setSelectedRepo}
                    disabled={repos.length === 0}
                >
                    <SelectTrigger
                        id="repo-select"
                        className="w-[180px] h-8 rounded-sm"
                    >
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
            </div>

            <div className="flex flex-col gap-1">
                <Label
                    htmlFor="arch-select"
                    className="text-xs font-medium text-muted-foreground"
                >
                    アーキテクチャ
                </Label>
                <Select
                    value={selectedArch || undefined}
                    onValueChange={setSelectedArch}
                    disabled={arches.length === 0}
                >
                    <SelectTrigger
                        id="arch-select"
                        className="w-[180px] h-8 rounded-sm"
                    >
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
            </div>
        </div>
    );
}
