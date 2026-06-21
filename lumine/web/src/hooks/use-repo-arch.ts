"use client";

import { atom, useAtom, useSetAtom } from "jotai";
import { useCallback, useEffect } from "react";
import { useAPIClient } from "@/components/lumine-provider";
import { useToast } from "@/hooks/use-toast";

const selectedRepoAtom = atom<string | null>(null);
const selectedArchAtom = atom<string | null>(null);
const reposAtom = atom<string[]>([]);
const archesAtom = atom<string[]>([]);

export function useRepoArch() {
    const [selectedRepo, setSelectedRepo] = useAtom(selectedRepoAtom);
    const [selectedArch, setSelectedArch] = useAtom(selectedArchAtom);
    const [repos, setRepos] = useAtom(reposAtom);
    const [arches, setArches] = useAtom(archesAtom);
    const setRepoOnly = useSetAtom(selectedRepoAtom);
    const setArchOnly = useSetAtom(selectedArchAtom);
    const api = useAPIClient();
    const { toast } = useToast();

    // Apply repo+arch together (e.g. when hydrating scope from the URL) so the
    // arch isn't transiently reset to the repo default before the URL value
    // lands. The arch-fetch effect below keeps a still-valid arch as-is.
    const setScope = useCallback(
        (repo: string, arch: string) => {
            setRepoOnly(repo);
            setArchOnly(arch);
        },
        [setRepoOnly, setArchOnly],
    );

    useEffect(() => {
        if (!api.endpoints.executable) return;
        if (repos.length > 0) return;

        const fetchRepos = async () => {
            try {
                const data = await api.fetchRepos();
                const repoList = Array.isArray(data) ? data : data.repos || [];
                setRepos(repoList);

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
        setScope,
        repos,
        arches,
    };
}
