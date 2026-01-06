import {
    Code2,
    ExternalLink,
    Github,
    Globe,
    Home,
    Package,
    Server,
    Sparkles,
    Twitter,
} from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";

export default function About() {
    return (
        <div className="min-h-screen">
            <section className="bg-muted/30 border-b border-border">
                <div className="container mx-auto px-4 sm:px-6 py-16 md:py-24">
                    <div className="max-w-4xl mx-auto text-center space-y-6">
                        <div className="inline-flex items-center gap-2 px-4 py-2 rounded-full bg-primary/10 border border-primary/20">
                            <Sparkles className="h-4 w-4 text-primary" />
                            <span className="text-sm font-medium text-primary">
                                About Lumine
                            </span>
                        </div>

                        <h1 className="text-5xl md:text-7xl font-bold tracking-tight text-primary">
                            Lumine Repository
                        </h1>

                        <p className="text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto leading-relaxed">
                            Ayakaバックエンドを利用した
                            <br className="hidden sm:block" />
                            Arch Linux向けパッケージリポジトリ
                        </p>
                    </div>
                </div>
            </section>

            <div className="container mx-auto px-4 sm:px-6 py-12 space-y-12 max-w-6xl">
                <Card className="border-primary/20 shadow-lg shadow-primary/5">
                    <CardHeader>
                        <CardTitle className="flex items-center gap-2 text-2xl">
                            <div className="p-2 rounded-lg bg-primary/10">
                                <Package className="w-5 h-5 text-primary" />
                            </div>
                            主な機能
                        </CardTitle>
                        <CardDescription className="text-base">
                            利用可能な機能
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div className="p-6 rounded-xl border-2 border-border hover:border-primary/50 transition-all duration-300">
                                <div className="flex items-start gap-4">
                                    <div className="p-3 rounded-lg bg-primary/10">
                                        <Package className="w-6 h-6 text-primary" />
                                    </div>
                                    <div className="flex-1">
                                        <h3 className="font-semibold text-lg mb-2">
                                            パッケージ管理
                                        </h3>
                                        <p className="text-sm text-muted-foreground leading-relaxed">
                                            パッケージの検索、閲覧、ダウンロードを
                                            <br />
                                            美しいUIで簡単に操作
                                        </p>
                                    </div>
                                </div>
                            </div>

                            <div className="p-6 rounded-xl border-2 border-border hover:border-secondary/50 transition-all duration-300">
                                <div className="flex items-start gap-4">
                                    <div className="p-3 rounded-lg bg-secondary/10">
                                        <Server className="w-6 h-6 text-secondary" />
                                    </div>
                                    <div className="flex-1">
                                        <h3 className="font-semibold text-lg mb-2">
                                            サーバー監視
                                        </h3>
                                        <p className="text-sm text-muted-foreground leading-relaxed">
                                            リアルタイムでサーバーの
                                            <br />
                                            ステータスを確認可能
                                        </p>
                                    </div>
                                </div>
                            </div>

                            <div className="p-6 rounded-xl border-2 border-border hover:border-accent/50 transition-all duration-300">
                                <div className="flex items-start gap-4">
                                    <div className="p-3 rounded-lg bg-accent/10">
                                        <Globe className="w-6 h-6 text-accent" />
                                    </div>
                                    <div className="flex-1">
                                        <h3 className="font-semibold text-lg mb-2">
                                            レスポンシブデザイン
                                        </h3>
                                        <p className="text-sm text-muted-foreground leading-relaxed">
                                            モバイル・デスクトップ対応の
                                            <br />
                                            モダンなUI/UX
                                        </p>
                                    </div>
                                </div>
                            </div>

                            <div className="p-6 rounded-xl border-2 border-border hover:border-primary/50 transition-all duration-300">
                                <div className="flex items-start gap-4">
                                    <div className="p-3 rounded-lg bg-primary/10">
                                        <Package className="w-6 h-6 text-primary" />
                                    </div>
                                    <div className="flex-1">
                                        <h3 className="font-semibold text-lg mb-2">
                                            パッケージアップロード
                                        </h3>
                                        <p className="text-sm text-muted-foreground leading-relaxed">
                                            GUIから簡単に
                                            <br />
                                            パッケージをアップロード
                                        </p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </CardContent>
                </Card>

                <Card className="border-secondary/20 shadow-lg shadow-secondary/5">
                    <CardHeader>
                        <CardTitle className="text-2xl">
                            プロジェクト構成
                        </CardTitle>
                        <CardDescription className="text-base">
                            3つのコンポーネントで構成
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-6">
                        <p className="text-muted-foreground leading-relaxed">
                            Arch
                            Linux向けのパッケージ配布・管理を目的としたオープンソースプロジェクト群です。
                        </p>
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                            <div className="p-6 border-2 border-primary/20 rounded-xl space-y-3 hover:border-primary/50 transition-all duration-300">
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-primary/10">
                                        <Server className="w-5 h-5 text-primary" />
                                    </div>
                                    <h3 className="font-bold text-lg">
                                        Ayaka CLI
                                    </h3>
                                </div>
                                <p className="text-sm text-muted-foreground leading-relaxed">
                                    パッケージビルドと管理のための
                                    <br />
                                    CLIツール
                                </p>
                            </div>

                            <div className="p-6 border-2 border-secondary/20 rounded-xl space-y-3 hover:border-secondary/50 transition-all duration-300">
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-secondary/10">
                                        <Server className="w-5 h-5 text-secondary" />
                                    </div>
                                    <h3 className="font-bold text-lg">
                                        Ayato Backend
                                    </h3>
                                </div>
                                <p className="text-sm text-muted-foreground leading-relaxed">
                                    パッケージ配信とアップロードを
                                    <br />
                                    処理するバックエンドAPI
                                </p>
                            </div>

                            <div className="p-6 border-2 border-accent/20 rounded-xl space-y-3 hover:border-accent/50 transition-all duration-300">
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-accent/10">
                                        <Globe className="w-5 h-5 text-accent" />
                                    </div>
                                    <h3 className="font-bold text-lg">
                                        Lumine Web
                                    </h3>
                                </div>
                                <p className="text-sm text-muted-foreground leading-relaxed">
                                    Webフロントエンド
                                    <br />
                                    (Next.js + React)
                                </p>
                            </div>
                        </div>
                    </CardContent>
                </Card>

                <Card className="border-accent/20 shadow-lg shadow-accent/5">
                    <CardHeader>
                        <CardTitle className="text-2xl">作者</CardTitle>
                        <CardDescription className="text-base">
                            山田ハヤオ
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <Link
                                href="https://www.hayao0819.com"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="group"
                            >
                                <Button
                                    variant="outline"
                                    className="w-full justify-between h-auto py-4 hover:border-primary/50 hover:bg-primary/5 transition-all"
                                >
                                    <span className="flex items-center gap-3">
                                        <Home className="w-5 h-5 text-primary" />
                                        <span className="font-medium">
                                            ホームページ
                                        </span>
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-50 group-hover:opacity-100 transition-opacity" />
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
                                    className="w-full justify-between h-auto py-4 hover:border-accent/50 hover:bg-accent/5 transition-all"
                                >
                                    <span className="flex items-center gap-3">
                                        <Twitter className="w-5 h-5 text-accent" />
                                        <span className="font-medium">
                                            Twitter (@Hayao0819)
                                        </span>
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-50 group-hover:opacity-100 transition-opacity" />
                                </Button>
                            </Link>
                        </div>
                    </CardContent>
                </Card>

                <Card className="border-secondary/20 shadow-lg shadow-secondary/5">
                    <CardHeader>
                        <CardTitle className="text-2xl">ソースコード</CardTitle>
                        <CardDescription className="text-base">
                            GitHubで公開中
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <Link
                                href="https://github.com/Hayao0819/Kamisato/tree/master/ayaka"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="group"
                            >
                                <Button
                                    variant="outline"
                                    className="w-full justify-between h-auto py-4 hover:border-primary/50 hover:bg-primary/5 transition-all"
                                >
                                    <span className="flex items-center gap-3">
                                        <Code2 className="w-5 h-5 text-primary" />
                                        <span className="font-medium">
                                            Ayaka CLI
                                        </span>
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-50 group-hover:opacity-100 transition-opacity" />
                                </Button>
                            </Link>

                            <Link
                                href="https://github.com/Hayao0819/Kamisato/tree/master/ayato"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="group"
                            >
                                <Button
                                    variant="outline"
                                    className="w-full justify-between h-auto py-4 hover:border-secondary/50 hover:bg-secondary/5 transition-all"
                                >
                                    <span className="flex items-center gap-3">
                                        <Code2 className="w-5 h-5 text-secondary" />
                                        <span className="font-medium">
                                            Ayato Backend
                                        </span>
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-50 group-hover:opacity-100 transition-opacity" />
                                </Button>
                            </Link>

                            <Link
                                href="https://github.com/Hayao0819/Kamisato/tree/master/lumine"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="group"
                            >
                                <Button
                                    variant="outline"
                                    className="w-full justify-between h-auto py-4 hover:border-accent/50 hover:bg-accent/5 transition-all"
                                >
                                    <span className="flex items-center gap-3">
                                        <Code2 className="w-5 h-5 text-accent" />
                                        <span className="font-medium">
                                            Lumine Web
                                        </span>
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-50 group-hover:opacity-100 transition-opacity" />
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
                                    className="w-full justify-between h-auto py-4 hover:border-primary/50 hover:bg-primary/5 transition-all"
                                >
                                    <span className="flex items-center gap-3">
                                        <Github className="w-5 h-5 text-primary" />
                                        <span className="font-medium">
                                            GitHub Repository
                                        </span>
                                    </span>
                                    <ExternalLink className="w-4 h-4 opacity-50 group-hover:opacity-100 transition-opacity" />
                                </Button>
                            </Link>
                        </div>
                        <div className="mt-4 p-4 border rounded-lg bg-card text-center">
                            <Link
                                href="https://github.com/Hayao0819/Kamisato/blob/main/LICENSE.txt"
                                className="text-sm text-primary hover:underline inline-flex items-center gap-1"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                MIT License
                                <ExternalLink className="w-3 h-3" />
                            </Link>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}
