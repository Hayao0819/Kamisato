"use client";

import { Menu, Package } from "lucide-react";
import Link from "next/link";
import { useMobileNav } from "@/hooks/use-console";

export function ConsoleTopbar() {
    const [, setOpen] = useMobileNav();
    return (
        <div className="sticky top-0 z-30 flex items-center gap-2 border-b border-border bg-background px-3 py-2 md:hidden">
            <button
                type="button"
                aria-label="メニューを開く"
                onClick={() => setOpen(true)}
                className="rounded-sm p-1.5 hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
                <Menu className="h-5 w-5" />
            </button>
            <Link href="/" className="flex items-center gap-2">
                <span className="flex h-7 w-7 items-center justify-center rounded-sm bg-primary">
                    <Package className="h-4 w-4 text-primary-foreground" />
                </span>
                <span className="text-[15px] font-bold">Lumine</span>
            </Link>
        </div>
    );
}
