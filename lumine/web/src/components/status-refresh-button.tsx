"use client";

import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function StatusRefreshButton({
    onRefresh,
    loading,
}: {
    onRefresh: () => void;
    loading?: boolean;
}) {
    return (
        <Button variant="outline" onClick={onRefresh} disabled={loading}>
            <RefreshCw className={cn("mr-2 h-4 w-4", loading && "animate-spin")} />
            更新
        </Button>
    );
}
