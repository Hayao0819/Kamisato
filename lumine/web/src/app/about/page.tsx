import { Footer } from "@/components/footer";
import Link from "next/link";

export default function About() {
    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6">
            <main className="space-y-8">
                <section>
                    <h1 className="text-2xl sm:text-3xl font-bold mb-2">
                        Lumine Web について
                    </h1>
                    <p className="text-base sm:text-lg text-muted-foreground mb-2">
                        <b>Lumine Web</b> は、Ayaka
                        バックエンドと連携して動作する Arch Linux
                        パッケージリポジトリのフロントエンドアプリケーションです。
                        パッケージの検索・一覧表示・サーバーステータス確認・バグ報告（モック）などが可能です。
                    </p>
                </section>
                <section>
                    <h2 className="text-xl sm:text-2xl font-bold mb-3 border-b-2 border-primary/60 pb-1 text-primary tracking-wide">
                        主な機能
                    </h2>
                    <ul className="list-disc list-inside space-y-1">
                        <li>パッケージ一覧の表示・検索</li>
                        <li>Ayaka バックエンドサーバーの状態確認</li>
                        <li>パッケージごとのバグ報告（モック機能）</li>
                        <li>APIサーバーURLの切り替え（右上の歯車アイコン）</li>
                        <li>モダンなUI・ダークモード対応</li>
                    </ul>
                </section>
                <section>
                    <h2 className="text-xl sm:text-2xl font-bold mb-3 border-b-2 border-primary/60 pb-1 text-primary tracking-wide">
                        Project Kamisato と関連プロジェクト
                    </h2>
                    <p className="text-base mb-2">
                        <b>Project Kamisato</b> は、Arch Linux
                        向けのパッケージ配布・管理を目的としたオープンソースプロジェクト群です。
                        Lumine
                        Webはその一部であり、以下の関連プロジェクトと連携しています。
                    </p>
                    <ul className="list-disc list-inside space-y-1">
                        <li>
                            <b>Ayaka</b>
                            ：パッケージリポジトリのバックエンドAPIサーバー
                        </li>
                        <li>
                            <b>Ayato</b>
                            ：パッケージのアップロード・管理用APIサーバー
                        </li>
                        <li>
                            <b>Lumine Web</b>：本フロントエンドアプリケーション
                        </li>
                    </ul>
                    <p className="text-xs text-muted-foreground mt-2">
                        詳細や他のプロジェクトについては{" "}
                        <Link
                            href="https://github.com/Hayao0819/Kamisato"
                            className="text-blue-600 hover:underline"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            Project Kamisato (GitHub)
                        </Link>{" "}
                        をご覧ください。
                    </p>
                </section>
                <section>
                    <h2 className="text-xl sm:text-2xl font-bold mb-3 border-b-2 border-primary/60 pb-1 text-primary tracking-wide">
                        リンク
                    </h2>
                    <ul className="list-disc list-inside space-y-1">
                        <li>
                            <Link
                                href="https://www.hayao0819.com"
                                className="text-blue-600 hover:underline"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                山田ハヤオのホームページ
                            </Link>
                        </li>
                        <li>
                            <Link
                                href="https://github.com/Hayao0819/Kamisato"
                                className="text-blue-600 hover:underline"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                ソースコード (GitHub)
                            </Link>
                        </li>
                        <li>
                            <Link
                                href="https://twitter.com/Hayao0819"
                                className="text-blue-600 hover:underline"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Twitter (@Hayao0819)
                            </Link>
                        </li>
                    </ul>
                </section>
                <section>
                    <h2 className="text-xl sm:text-2xl font-bold mb-3 border-b-2 border-primary/60 pb-1 text-primary tracking-wide">
                        ライセンス
                    </h2>
                    <p className="text-sm">
                        本プロジェクトは{" "}
                        <Link
                            href="https://github.com/Hayao0819/Kamisato/blob/main/LICENSE.txt"
                            className="text-blue-600 hover:underline"
                            target="_blank"
                            rel="noopener noreferrer"
                        >
                            MIT License
                        </Link>{" "}
                        で公開されています。
                    </p>
                </section>
            </main>
            <Footer />
        </div>
    );
}
