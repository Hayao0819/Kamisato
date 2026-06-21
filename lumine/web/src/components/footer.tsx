import { Github } from "lucide-react";
import Link from "next/link";

export function Footer() {
    return (
        <footer className="w-full mt-auto arch-titlebar text-arch-bar-foreground/80">
            <div className="container mx-auto py-5 px-4 md:px-6">
                <div className="flex flex-col md:flex-row items-center justify-between gap-3 text-xs">
                    <div className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1">
                        <span className="font-semibold text-arch-bar-foreground">
                            Lumine Repository
                        </span>
                        <span className="text-arch-bar-foreground/40">|</span>
                        <span>© 2025 山田ハヤオ</span>
                        <span className="hidden md:inline text-arch-bar-foreground/40">
                            |
                        </span>
                        <span className="hidden md:inline">
                            Ayaka CLI · Ayato Backend · Lumine Web
                        </span>
                    </div>
                    <Link
                        href="https://github.com/Hayao0819/Kamisato"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-1.5 hover:text-primary transition-colors"
                    >
                        <Github className="h-4 w-4" />
                        <span>GitHub</span>
                    </Link>
                </div>
            </div>
        </footer>
    );
}
