import { ExternalLink } from "lucide-react";
import Link from "next/link";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";

interface RepoLink {
    label: string;
    href: string;
    note: string;
}

const sourceLinks: RepoLink[] = [
    {
        label: "Kamisato リポジトリ",
        href: "https://github.com/Hayao0819/Kamisato",
        note: "全コンポーネントのソースコード (モノレポ)",
    },
    {
        label: "MIT License",
        href: "https://github.com/Hayao0819/Kamisato/blob/master/LICENSE.txt",
        note: "ライセンス全文",
    },
];

const adminLinks: RepoLink[] = [
    {
        label: "山田ハヤオ",
        href: "https://www.hayao0819.com",
        note: "管理者のホームページ",
    },
    {
        label: "@Hayao0819",
        href: "https://twitter.com/Hayao0819",
        note: "Twitter",
    },
];

function LinkList({ items }: { items: RepoLink[] }) {
    return (
        <ul className="space-y-3">
            {items.map((item) => (
                <li key={item.href} className="flex flex-col gap-0.5">
                    <Link
                        href={item.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex w-fit items-center gap-1 font-medium text-link hover:underline"
                    >
                        {item.label}
                        <ExternalLink className="h-3.5 w-3.5" />
                    </Link>
                    <span className="text-sm text-muted-foreground">
                        {item.note}
                    </span>
                </li>
            ))}
        </ul>
    );
}

export default function About() {
    return (
        <PageContainer
            measure="prose"
            header={<PageHeader title="このサイトについて" />}
        >
            <div className="space-y-10">
                <section className="space-y-4">
                    <h2 className="text-lg font-semibold tracking-tight">
                        ソースコード
                    </h2>
                    <LinkList items={sourceLinks} />
                </section>

                <section className="space-y-4">
                    <h2 className="text-lg font-semibold tracking-tight">
                        管理者
                    </h2>
                    <LinkList items={adminLinks} />
                </section>
            </div>
        </PageContainer>
    );
}
