"use client";

import { atom, useAtom } from "jotai";
import { useEffect } from "react";
import { useAPIClient } from "@/components/lumine-provider";
import { useToast } from "@/hooks/use-toast";

// Atoms for global state
const selectedRepoAtom = atom<string | null>(null);
const selectedArchAtom = atom<string | null>(null);
const reposAtom = atom<string[]>([]);
const archesAtom = atom<string[]>([]);

/**
 * Custom hook for managing repository and architecture selection globally
 */
export function useRepoArch() {
    const [selectedRepo, setSelectedRepo] = useAtom(selectedRepoAtom);
    const [selectedArch, setSelectedArch] = useAtom(selectedArchAtom);
    const [repos, setRepos] = useAtom(reposAtom);
    const [arches, setArches] = useAtom(archesAtom);
    const api = useAPIClient();
    const { toast } = useToast();

    // Fetch repositories on mount
    useEffect(() => {
        if (!api.endpoints.executable) return;
        if (repos.length > 0) return; // Already loaded

        const fetchRepos = async () => {
            try {
                const data = await api.fetchRepos();
                const repoList = Array.isArray(data) ? data : data.repos || [];
                setRepos(repoList);

                // Auto-select first repo if not already selected
                if (repoList.length > 0 && !selectedRepo) {
                    setSelectedRepo(repoList[0]);
                }
            } catch (error) {
                toast({
                    title: "リポジトリ取得エラー",
                    description:
                        error instanceof Error ? error.message : String(error),
                    variant: "destructive",
                });
            }
        };

        fetchRepos();
    }, [api, repos.length, selectedRepo, setRepos, setSelectedRepo, toast]);

    // Fetch architectures when selectedRepo changes
    useEffect(() => {
        if (!api.endpoints.executable) return;
        if (!selectedRepo) {
            setArches([]);
            setSelectedArch(null);
            return;
        }

        const fetchArches = async () => {
            try {
                const data = await api.fetchArches(selectedRepo);
                const archList = Array.isArray(data) ? data : data.arches || [];
                setArches(archList);

                // Auto-select first arch if not already selected or if current selection is invalid
                if (
                    archList.length > 0 &&
                    (!selectedArch || !archList.includes(selectedArch))
                ) {
                    setSelectedArch(archList[0]);
                }
            } catch (error) {
                toast({
                    title: "アーキテクチャ取得エラー",
                    description:
                        error instanceof Error ? error.message : String(error),
                    variant: "destructive",
                });
                setArches([]);
                setSelectedArch(null);
            }
        };

        fetchArches();
    }, [selectedRepo, api, selectedArch, setArches, setSelectedArch, toast]);

    return {
        selectedRepo,
        setSelectedRepo,
        selectedArch,
        setSelectedArch,
        repos,
        arches,
    };
}
