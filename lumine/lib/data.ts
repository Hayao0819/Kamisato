// モックデータ

// パッケージデータ
export interface Package {
    id: string;
    name: string;
    version: string;
    description: string;
    maintainer: string;
    lastUpdated: string;
    dependencies: number;
    size: string;
}

export const packages: Package[] = [
    {
        id: "1",
        name: "linux",
        version: "6.6.7-arch1-1",
        description: "Linux カーネル",
        maintainer: "Arch Linux Team",
        lastUpdated: "2023-12-15",
        dependencies: 5,
        size: "150.5 MB",
    },
    {
        id: "2",
        name: "firefox",
        version: "121.0-1",
        description: "ウェブブラウザ",
        maintainer: "Jan de Groot",
        lastUpdated: "2023-12-10",
        dependencies: 45,
        size: "85.2 MB",
    },
    {
        id: "3",
        name: "neovim",
        version: "0.9.4-1",
        description: "Vim ベースのテキストエディタ",
        maintainer: "Antonio Rojas",
        lastUpdated: "2023-11-28",
        dependencies: 12,
        size: "24.7 MB",
    },
    {
        id: "4",
        name: "nodejs",
        version: "20.10.0-1",
        description: "JavaScript ランタイム環境",
        maintainer: "Felix Yan",
        lastUpdated: "2023-12-05",
        dependencies: 8,
        size: "36.1 MB",
    },
    {
        id: "5",
        name: "python",
        version: "3.11.6-1",
        description: "Python プログラミング言語",
        maintainer: "Angel Velasquez",
        lastUpdated: "2023-11-20",
        dependencies: 3,
        size: "18.9 MB",
    },
    {
        id: "6",
        name: "gimp",
        version: "2.10.36-1",
        description: "画像編集ソフト",
        maintainer: "Levente Polyak",
        lastUpdated: "2023-12-01",
        dependencies: 78,
        size: "120.3 MB",
    },
    {
        id: "7",
        name: "vlc",
        version: "3.0.20-1",
        description: "マルチメディアプレーヤー",
        maintainer: "Maxime Gauduin",
        lastUpdated: "2023-12-08",
        dependencies: 65,
        size: "45.8 MB",
    },
    {
        id: "8",
        name: "docker",
        version: "1:24.0.7-1",
        description: "コンテナ管理ツール",
        maintainer: "Sébastien Luttringer",
        lastUpdated: "2023-12-12",
        dependencies: 15,
        size: "102.4 MB",
    },
    {
        id: "9",
        name: "git",
        version: "2.43.0-1",
        description: "バージョン管理システム",
        maintainer: "Christian Hesse",
        lastUpdated: "2023-12-07",
        dependencies: 6,
        size: "7.8 MB",
    },
    {
        id: "10",
        name: "libreoffice-fresh",
        version: "7.6.3-1",
        description: "オフィススイート",
        maintainer: "Andreas Radke",
        lastUpdated: "2023-11-25",
        dependencies: 120,
        size: "245.6 MB",
    },
];

// サーバーステータスデータ
export interface ServerStatus {
    id: string;
    name: string;
    status: "online" | "offline" | "maintenance" | "syncing";
    uptime: string;
    lastSync: string;
    packages: number;
    load: number;
    diskUsage: number;
}

export const servers: ServerStatus[] = [
    {
        id: "1",
        name: "メインサーバー",
        status: "online",
        uptime: "45日 12時間 32分",
        lastSync: "2023-12-15 08:45:12",
        packages: 12458,
        load: 0.45,
        diskUsage: 78,
    },
    {
        id: "2",
        name: "ミラー1 (アジア)",
        status: "online",
        uptime: "32日 5時間 18分",
        lastSync: "2023-12-15 09:12:45",
        packages: 12458,
        load: 0.32,
        diskUsage: 65,
    },
    {
        id: "3",
        name: "ミラー2 (ヨーロッパ)",
        status: "syncing",
        uptime: "12日 8時間 55分",
        lastSync: "2023-12-15 07:30:22",
        packages: 12445,
        load: 0.87,
        diskUsage: 72,
    },
    {
        id: "4",
        name: "ミラー3 (北米)",
        status: "online",
        uptime: "28日 14時間 05分",
        lastSync: "2023-12-15 08:55:10",
        packages: 12458,
        load: 0.28,
        diskUsage: 61,
    },
    {
        id: "5",
        name: "テストサーバー",
        status: "maintenance",
        uptime: "2日 4時間 15分",
        lastSync: "2023-12-14 15:20:45",
        packages: 12458,
        load: 0.05,
        diskUsage: 45,
    },
];
