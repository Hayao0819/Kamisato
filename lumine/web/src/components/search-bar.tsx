"use client";

import type React from "react";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Search } from "lucide-react";

interface SearchBarProps {
    onSearch: (query: string) => void;
}

export function SearchBar({ onSearch }: SearchBarProps) {
    const [query, setQuery] = useState("");

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        onSearch(query);
    };

    return (
        <form
            onSubmit={handleSubmit}
            className="flex w-full max-w-sm items-center space-x-2"
        >
            <Input
                type="text"
                placeholder="パッケージを検索..."
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="flex-1"
            />
            <Button type="submit" size="icon">
                <Search className="h-4 w-4" />
                <span className="sr-only">検索</span>
            </Button>
        </form>
    );
}
