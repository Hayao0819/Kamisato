import { Footer } from "@/components/footer";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import {
    ExternalLink,
    Github,
    Globe,
    Home,
    Package,
    Server,
    Twitter,
} from "lucide-react";
import Link from "next/link";

export default function About() {
    return (
        <div className="container mx-auto py-4 sm:py-8 px-4 sm:px-6">
            <main className="space-y-8 max-w-5xl mx-auto">
                {/* Hero Section */}
                <section className="text-center space-y-4 py-8">
                    <h1 className="text-4xl sm:text-5xl font-bold bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
                        Lumine Web
                    </h1>
                    <p className="text-lg sm:text-xl text-muted-foreground max-w-2xl mx-auto">
                        Arch Linux パッケージリポジトリのモダンなフロントエンド
                    </p>
                    <div className="flex gap-2 justify-center flex-wrap">
                        <Badge variant="secondary">Next.js 15</Badge>
                        <Badge variant="secondary">React 19</Badge>
                        <Badge variant="secondary">TypeScript</Badge>
                        <Badge variant="secondary">Tailwind CSS</Badge>
                    </div>
                </section>

                {/* Main Features */}
                <Card>
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2">
                            <Package className="w-5 h-5" />
                            主な機能
                        </CardTitle>
                        <CardDescription>
                            Lumine Webで利用可能な機能一覧
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="flex items-start gap-3 p-3 rounded-lg border bg-card hover:bg-accent/50 transition-colors">
                                <div className="mt-1">
                                    <Package className="w-5 h-5 text-primary" />
                                </div>
                                <div>
                                    <h3 className="font-semibold">
                                        パッケージ管理
                                    </h3>
                                    <p className="text-sm text-muted-foreground">
                                        パッケージの検索、閲覧、ダウンロード
                                    </p>
                                </div>
                            </div>
                            <div className="flex items-start gap-3 p-3 rounded-lg border bg-card hover:bg-accent/50 transition-colors">
                                <div className="mt-1">
                                    <Server className="w-5 h-5 text-primary" />
                                </div>
                                <div>
                                    <h3 className="font-semibold">
                                        サーバー監視
                                    </h3>
                                    <p className="text-sm text-muted-foreground">
                                        リアルタイムのサーバーステータス確認
                                    </p>
                                </div>
                            </div>
                            <div className="flex items-start gap-3 p-3 rounded-lg border bg-card hover:bg-accent/50 transition-colors">
                                <div className="mt-1">
                                    <Globe className="w-5 h-5 text-primary" />
                                </div>
                                <div>
                                    <h3 className="font-semibold">
                                        レスポンシブデザイン
                                    </h3>
                                    <p className="text-sm text-muted-foreground">
                                        モバイル・デスクトップ対応のモダンUI
                                    </p>
                                </div>
                            </div>
                            <div className="flex items-start gap-3 p-3 rounded-lg border bg-card hover:bg-accent/50 transition-colors">
                                <div className="mt-1">
                                    <Package className="w-5 h-5 text-primary" />
                                </div>
                                <div>
                                    <h3 className="font-semibold">
                                        パッケージアップロード
                                    </h3>
                                    <p className="text-sm text-muted-foreground">
                                        GUIからパッケージを簡単にアップロード
                                    </p>
                                </div>
                            </div>
                        </div>
                    </CardContent>
                </Card>

                {/* Project Kamisato */}
                <Card>
                    <CardHeader>
                        <CardTitle>Project Kamisato</CardTitle>
                        <CardDescription>
                            Arch Linux向けパッケージ配布・管理プロジェクト
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <p className="text-muted-foreground">
                            <strong>Project Kamisato</strong>は、Arch
                            Linux向けのパッケージ配布・管理を目的としたオープンソースプロジェクト群です。
                            以下のコンポーネントで構成されています。
                        </p>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            <div className="p-4 border rounded-lg space-y-2">
                                <div className="flex items-center gap-2">
                                    <Server className="w-4 h-4 text-primary" />
                                    <h3 className="font-semibold">Ayaka</h3>
                                </div>
                                <p className="text-sm text-muted-foreground">
                                    パッケージリポジトリのバックエンドAPIサーバー
                                </p>
                            </div>
                            <div className="p-4 border rounded-lg space-y-2">
                                <div className="flex items-center gap-2">
                                    <Server className="w-4 h-4 text-primary" />
                                    <h3 className="font-semibold">Ayato</h3>
                                </div>
                                <p className="text-sm text-muted-foreground">
                                    パッケージアップロード・管理用APIサーバー
                                </p>
                            </div>
                            <div className="p-4 border rounded-lg space-y-2">
                                <div className="flex items-center gap-2">
                                    <Globe className="w-4 h-4 text-primary" />
                                    <h3 className="font-semibold">
                                        Lumine Web
                                    </h3>
                                </div>
                                <p className="text-sm text-muted-foreground">
                                    本フロントエンドアプリケーション
                                </p>
                            </div>
                        </div>
                    </CardContent>
                </Card>

                {/* Links */}
                <Card>
                    <CardHeader>
                        <CardTitle>リンク</CardTitle>
                        <CardDescription>
                            関連リンクとソーシャルメディア
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="flex flex-col gap-3">
                            <Link
                                href="https://www.hayao0819.com"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="group"
                            >
                                <Button
                                    variant="outline"
                                    className="w-full justify-between"
                                >
                                    <span className="flex items-center gap-2">
                                        <Home className="w-4 h-4" />
                                        山田ハヤオのホームページ
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-0 group-hover:opacity-100 transition-opacity" />
                                </Button>
                            </Link>
                            <Link
                                href="https://github.com/Hayao0819/Kamisato"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="group"
                            >
                                <Button
                                    variant="outline"
                                    className="w-full justify-between"
                                >
                                    <span className="flex items-center gap-2">
                                        <Github className="w-4 h-4" />
                                        ソースコード (GitHub)
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-0 group-hover:opacity-100 transition-opacity" />
                                </Button>
                            </Link>
                            <Link
                                href="https://twitter.com/Hayao0819"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="group"
                            >
                                <Button
                                    variant="outline"
                                    className="w-full justify-between"
                                >
                                    <span className="flex items-center gap-2">
                                        <Twitter className="w-4 h-4" />
                                        Twitter (@Hayao0819)
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-0 group-hover:opacity-100 transition-opacity" />
                                </Button>
                            </Link>
                        </div>
                    </CardContent>
                </Card>

                {/* License */}
                <Card>
                    <CardHeader>
                        <CardTitle>ライセンス</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <p className="text-sm text-muted-foreground">
                            本プロジェクトは{" "}
                            <Link
                                href="https://github.com/Hayao0819/Kamisato/blob/main/LICENSE.txt"
                                className="text-primary hover:underline inline-flex items-center gap-1"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                MIT License
                                <ExternalLink className="w-3 h-3" />
                            </Link>{" "}
                            で公開されています。
                        </p>
                    </CardContent>
                </Card>
            </main>
            <Footer />
        </div>
    );
}
