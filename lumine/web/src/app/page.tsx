import { PackageTable } from "@/components/package-table";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ServerIcon } from "lucide-react";
import { getAllPkgsEndpoint } from "@/lib/api";
import { PackageInfo, PacmanPkgsResponse } from "@/lib/types";

async function getPackages(): Promise<PackageInfo[]> {
  const res = await fetch(getAllPkgsEndpoint("repo", "x86_64")); // Assuming "myrepo" and "x86_64" for now
  if (!res.ok) {
    // This will activate the closest `error.js` Error Boundary
    throw new Error('Failed to fetch packages');
  }
  const data: PacmanPkgsResponse = await res.json();
  if (!Array.isArray(data.packages)) {
    console.error("Fetched data.packages is not an array:", data.packages);
    return []; // Return empty array if data.packages is not an array
  }
  return data.packages;
}

export default async function Home() {
    const packages: PackageInfo[] = await getPackages();

    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6">
            <header className="mb-6 sm:mb-8">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-4">
                    <h1 className="text-2xl sm:text-3xl font-bold">
                        Lumine (非公式) パッケージリポジトリ
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
                    用の非公式パッケージリポジトリです。Lumine アプリケーションを通じて、パッケージを検索、ダウンロードできます。
                </p>
            </header>

            <main>
                <PackageTable packages={packages} />
            </main>

            <footer className="mt-8 sm:mt-12 text-center text-xs sm:text-sm text-muted-foreground py-4">
                <p>© 2023 山田ハヤオ / Lumine</p>
            </footer>
        </div>
    );
}
