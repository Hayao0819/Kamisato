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
        <div className="flex space-x-4">
            <Select
                value={selectedRepo || undefined}
                onValueChange={setSelectedRepo}
                disabled={repos.length === 0}
            >
                <SelectTrigger className="w-[180px]">
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

            <Select
                value={selectedArch || undefined}
                onValueChange={setSelectedArch}
                disabled={arches.length === 0}
            >
                <SelectTrigger className="w-[180px]">
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
    );
}
