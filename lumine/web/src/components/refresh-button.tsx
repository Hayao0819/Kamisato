"use client";

import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

export function RefreshButton() {
    return (
        <Button
            variant="outline"
            className="flex-1 sm:flex-auto"
            onClick={() => window.location.reload()}
        >
            <RefreshCw className="h-4 w-4 mr-2" />
            更新
        </Button>
    );
}
