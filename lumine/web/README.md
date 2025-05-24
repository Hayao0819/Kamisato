# Lumine Web

Lumine Web は、Ayaka バックエンドのArch Linux パッケージリポジトリのためのフロントエンドアプリケーションです。

## 機能

- パッケージ一覧の表示
- パッケージの検索
- Ayaka バックエンドのサーバーステータス表示
- パッケージに関するバグ報告（モック機能）

## 使用技術

- Next.js
- React
- TypeScript
- Tailwind CSS
- shadcn/ui
- Lucide React (アイコン)
- next-themes (テーマ切り替え)
- embla-carousel-react (カルーセル)
- sonner (トースト通知)
- class-variance-authority (スタイルユーティリティ)
- @radix-ui/react-* (UI プリミティブ)

## セットアップ

1. このリポジトリをクローンします。
2. `lumine/web` ディレクトリに移動します。

    ```bash
    cd lumine/web
    ```

3. 依存関係をインストールします。pnpm を使用している場合:

    ```bash
    pnpm install
    ```

    npm または yarn を使用している場合は、それぞれのコマンドを使用してください。
4. 環境変数を設定します。`.env.local` ファイルを作成し、Ayaka バックエンドの URL を設定します。

    ```env
    NEXT_PUBLIC_API_BASE_URL=http://localhost:9000
    ```

    必要に応じて URL を変更してください。

## 開発サーバーの起動

開発サーバーを起動するには、以下のコマンドを実行します。

```bash
pnpm dev
```

または

```bash
npm run dev
```

```bash
yarn dev
```

アプリケーションは `http://localhost:3000` で利用可能になります。

## プロジェクト構造

- `app/`: Next.js の App Router によるページルーティング
  - `layout.tsx`: ルートレイアウト
  - `page.tsx`: パッケージ一覧ページ
  - `about/page.tsx`: Lumine についてのページ
  - `server-status/page.tsx`: サーバーステータス表示ページ
- `components/`: 再利用可能なコンポーネント
  - `ui/`: shadcn/ui から取得した UI コンポーネント
  - その他のコンポーネント（`package-table.tsx`, `search-bar.tsx` など）
- `hooks/`: カスタムフック
- `lib/`: ユーティリティ関数や型定義
  - `api.ts`: バックエンド API 関連
  - `types.ts`: 型定義
  - `utils.ts`: その他のユーティリティ
- `styles/`: グローバルスタイル

## ライセンス

[LICENSE.txt](https://github.com/Hayao0819/Kamisato/blob/main/LICENSE.txt) を参照してください。
