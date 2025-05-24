import Link from "next/link";
import { Button } from "@/components/ui/button";
import { ArrowLeft } from "lucide-react";

export default function About() {
    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6">
            <header className="mb-6 sm:mb-8">
                <div className="flex justify-between items-center mb-4">
                    <h1 className="text-2xl sm:text-3xl font-bold">
                        Lumine について
                    </h1>
                    <Link href="/">
                        <Button variant="outline">
                            <ArrowLeft className="h-4 w-4 mr-2" />
                            戻る
                        </Button>
                    </Link>
                </div>
                <p className="text-sm sm:text-base text-muted-foreground">
                    Lumine は、非公式の Arch Linux パッケージリポジトリのフロントエンドアプリケーションです。
                    Ayaka バックエンドと連携して動作します。
                </p>
            </header>

            <main className="space-y-6">
                <div>
                    <h2 className="text-xl sm:text-2xl font-semibold mb-2">リンク</h2>
                    <ul className="list-disc list-inside space-y-1">
                        <li>
                            <Link href="https://www.hayao0819.com" className="text-blue-600 hover:underline" target="_blank" rel="noopener noreferrer">
                                山田ハヤオのホームページ
                            </Link>
                        </li>
                        <li>
                            <Link href="https://github.com/Hayao0819/Kamisato" className="text-blue-600 hover:underline" target="_blank" rel="noopener noreferrer">
                                ソースコード (GitHub)
                            </Link>
                        </li>
                        <li>
                            <Link href="https://twitter.com/Hayao0819" className="text-blue-600 hover:underline" target="_blank" rel="noopener noreferrer">
                                Twitter (@Hayao0819)
                            </Link>
                        </li>
                    </ul>
                </div>
            </main>

            <footer className="mt-8 sm:mt-12 text-center text-xs sm:text-sm text-muted-foreground py-4">
                <p>© 2023 山田ハヤオ / Lumine</p>
            </footer>
        </div>
    );
}
