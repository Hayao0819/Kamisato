"use client";

import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useEffect, useState } from "react";
import { useToast } from "@/hooks/use-toast";
import { useAPIClient } from "./lumine-provider";

interface RepoArchSelectorProps {
    onSelect: (repo: string, arch: string) => void;
}

export function RepoArchSelector({ onSelect }: RepoArchSelectorProps) {
    const [repos, setRepos] = useState<string[]>([]);
    const [selectedRepo, setSelectedRepo] = useState<string | null>(null);
    const [arches, setArches] = useState<string[]>([]);
    const [selectedArch, setSelectedArch] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const { toast } = useToast();
    const api = useAPIClient();

    // Fetch repositories on component mount
    useEffect(() => {
        if (!api.endpoints.executable) return;
        const fetchRepos = async () => {
            try {
                const data: string[] = await api.fetchRepos();
                setRepos(data);
                setError(null);
                if (data.length > 0) {
                    setSelectedRepo(data[0]);
                }
            } catch (error) {
                setError("リポジトリ一覧の取得に失敗しました");
                setRepos([]);
                setSelectedRepo(null);
                toast({
                    title: "リポジトリ取得エラー",
                    description:
                        error instanceof Error ? error.message : String(error),
                    variant: "destructive",
                });
            }
        };
        fetchRepos();
    }, [toast, api.endpoints.executable]);

    // Fetch architectures when selectedRepo changes
    useEffect(() => {
        if (!api.endpoints.executable) return;
        if (selectedRepo) {
            const fetchArches = async () => {
                try {
                    const data: string[] = await api.fetchArches(selectedRepo);
                    setArches(data);
                    setError(null);
                    if (data.length > 0) {
                        setSelectedArch(data[0]);
                    } else {
                        setSelectedArch(null);
                    }
                } catch (error) {
                    setError("アーキテクチャ一覧の取得に失敗しました");
                    setArches([]);
                    setSelectedArch(null);
                    toast({
                        title: "アーキテクチャ取得エラー",
                        description:
                            error instanceof Error ? error.message : String(error),
                        variant: "destructive",
                    });
                }
            };
            fetchArches();
        } else {
            setArches([]);
            setSelectedArch(null);
        }
    }, [selectedRepo, toast, api.endpoints.executable]);

    // Call onSelect when both repo and arch are selected
    useEffect(() => {
        if (selectedRepo && selectedArch) {
            onSelect(selectedRepo, selectedArch);
        }
    }, [selectedRepo, selectedArch, onSelect]);

    return (
        <div className="flex space-x-4">
            <Select
                value={selectedRepo || ""}
                onValueChange={(value) => setSelectedRepo(value)}
                disabled={!!error || repos.length === 0}
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
                value={selectedArch || ""}
                onValueChange={(value) => setSelectedArch(value)}
                disabled={!!error || arches.length === 0}
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
