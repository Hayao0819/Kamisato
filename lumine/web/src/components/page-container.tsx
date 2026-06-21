import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export interface PageContainerProps {
    children: ReactNode;
    measure?: "full" | "form" | "prose";
    header?: ReactNode;
}

export function PageContainer({
    children,
    measure = "full",
    header,
}: PageContainerProps) {
    return (
        <div className="w-full mx-auto max-w-(--content-max) px-4 sm:px-6 lg:px-8 pt-6 sm:pt-8 pb-10">
            {header}
            <div
                className={cn(
                    "mt-6 sm:mt-8",
                    measure === "form" && "max-w-(--measure-form)",
                    measure === "prose" && "max-w-(--measure-prose)",
                )}
            >
                {children}
            </div>
        </div>
    );
}
