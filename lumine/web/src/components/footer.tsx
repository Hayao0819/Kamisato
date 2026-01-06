import { Github } from "lucide-react";
import Link from "next/link";

export function Footer() {
    return (
        <footer className="w-full border-t border-border mt-auto bg-card">
            <div className="container mx-auto py-6 px-4 md:px-6">
                <div className="flex flex-col md:flex-row items-center justify-between gap-4">
                    <div className="flex items-center gap-4 text-xs text-muted-foreground">
                        <span>© 2025 山田ハヤオ</span>
                        <span className="hidden md:inline">•</span>
                        <span className="hidden md:inline">
                            Ayaka CLI • Ayato Backend • Lumine Web
                        </span>
                    </div>
                    <Link
                        href="https://github.com/Hayao0819/Kamisato"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-2 text-xs text-muted-foreground hover:text-primary transition-colors"
                    >
                        <Github className="h-4 w-4" />
                        <span>GitHub</span>
                    </Link>
                </div>
            </div>
        </footer>
    );
}
