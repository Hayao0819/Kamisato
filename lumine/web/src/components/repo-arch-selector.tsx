"use client";

import { useState, useEffect } from "react";
import { getReposEndpoint, getArchesEndpoint } from "@/lib/api";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";

interface RepoArchSelectorProps {
    onSelect: (repo: string, arch: string) => void;
}

export function RepoArchSelector({ onSelect }: RepoArchSelectorProps) {
    const [repos, setRepos] = useState<string[]>([]);
    const [selectedRepo, setSelectedRepo] = useState<string | null>(null);
    const [arches, setArches] = useState<string[]>([]);
    const [selectedArch, setSelectedArch] = useState<string | null>(null);

    // Fetch repositories on component mount
    useEffect(() => {
        const fetchRepos = async () => {
            try {
                const response = await fetch(getReposEndpoint());
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                const data: string[] = await response.json();
                setRepos(data);
                if (data.length > 0) {
                    setSelectedRepo(data[0]); // Select the first repo by default
                }
            } catch (error) {
                console.error("Failed to fetch repositories:", error);
                // TODO: Display error message to user
            }
        };

        fetchRepos();
    }, []);

    // Fetch architectures when selectedRepo changes
    useEffect(() => {
        if (selectedRepo) {
            const fetchArches = async () => {
                try {
                    const response = await fetch(getArchesEndpoint(selectedRepo));
                    if (!response.ok) {
                        throw new Error(`HTTP error! status: ${response.status}`);
                    }
                    const data: string[] = await response.json();
                    setArches(data);
                    if (data.length > 0) {
                        setSelectedArch(data[0]); // Select the first arch by default
                    } else {
                        setSelectedArch(null);
                    }
                } catch (error) {
                    console.error(`Failed to fetch architectures for ${selectedRepo}:`, error);
                    setArches([]);
                    setSelectedArch(null);
                    // TODO: Display error message to user
                }
            };

            fetchArches();
        } else {
            setArches([]);
            setSelectedArch(null);
        }
    }, [selectedRepo]);

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
                value={selectedArch || ""}
                onValueChange={(value) => setSelectedArch(value)}
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
