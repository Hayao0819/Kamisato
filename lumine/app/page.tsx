import { packages } from "@/lib/data";
import { PackageTable } from "@/components/package-table";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ServerIcon } from "lucide-react";

export default function Home() {
    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6">
            <header className="mb-6 sm:mb-8">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-4">
                    <h1 className="text-2xl sm:text-3xl font-bold">
                        Pacman パッケージリポジトリ
                    </h1>
                    <Link href="/server-status">
                        <Button variant="outline" className="w-full sm:w-auto">
                            <ServerIcon className="h-4 w-4 mr-2" />
                            サーバーステータス
                        </Button>
                    </Link>
                </div>
                <p className="text-sm sm:text-base text-muted-foreground">
                    Arch Linux
                    用の公式パッケージリポジトリです。最新のパッケージを検索、ダウンロードできます。
                </p>
            </header>

            <main>
                <PackageTable packages={packages} />
            </main>

            <footer className="mt-8 sm:mt-12 text-center text-xs sm:text-sm text-muted-foreground py-4">
                <p>© 2023 Pacman パッケージリポジトリ</p>
            </footer>
        </div>
    );
}
