import Link from "next/link";
import type { ReactNode } from "react";

export interface Breadcrumb {
    label: string;
    href?: string;
}

export interface PageHeaderProps {
    title: string;
    description?: ReactNode;
    breadcrumbs?: Breadcrumb[];
    actions?: ReactNode;
}

export function PageHeader({
    title,
    description,
    breadcrumbs,
    actions,
}: PageHeaderProps) {
    return (
        <header className="w-full">
            {breadcrumbs && breadcrumbs.length > 0 && (
                <nav
                    aria-label="パンくずリスト"
                    className="mb-2 text-sm text-muted-foreground"
                >
                    <ol className="flex flex-wrap items-center gap-1.5">
                        {breadcrumbs.map((crumb, i) => {
                            const isLast = i === breadcrumbs.length - 1;
                            return (
                                <li
                                    key={`${crumb.label}-${i}`}
                                    className="flex items-center gap-1.5"
                                >
                                    {i > 0 && (
                                        <span
                                            aria-hidden="true"
                                            className="text-muted-foreground/50"
                                        >
                                            /
                                        </span>
                                    )}
                                    {crumb.href && !isLast ? (
                                        <Link
                                            href={crumb.href}
                                            className="hover:text-foreground hover:underline"
                                        >
                                            {crumb.label}
                                        </Link>
                                    ) : (
                                        <span
                                            aria-current={
                                                isLast ? "page" : undefined
                                            }
                                            className={
                                                isLast
                                                    ? "text-foreground"
                                                    : undefined
                                            }
                                        >
                                            {crumb.label}
                                        </span>
                                    )}
                                </li>
                            );
                        })}
                    </ol>
                </nav>
            )}
            <div className="flex items-start justify-between gap-4 min-h-9">
                <h1 className="text-2xl font-semibold tracking-tight">
                    {title}
                </h1>
                {actions && (
                    <div className="flex items-center gap-2 shrink-0">
                        {actions}
                    </div>
                )}
            </div>
            {description && (
                <p className="mt-1 text-sm text-muted-foreground max-w-[600px]">
                    {description}
                </p>
            )}
        </header>
    );
}
