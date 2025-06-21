
# Lumine Web

Lumine Web is a frontend application for the Arch Linux package repository backend
(Ayaka).

## Features

- Display package list
- Search packages
- Show Ayaka backend server status
- Bug reporting for packages (mock function)

## Technologies Used

- Next.js
- React
- TypeScript
- Tailwind CSS
- shadcn/ui
- Lucide React (icons)
- next-themes (theme switching)
- embla-carousel-react (carousel)
- sonner (toast notification)
- class-variance-authority (style utility)
- @radix-ui/react-* (UI primitives)

## Setup

1. Clone this repository.
2. Move to the `lumine/web` directory.

    ```bash
    cd lumine/web
    ```

3. Install dependencies. If you use pnpm:

    ```bash
    pnpm install
    ```

    If you use npm or yarn, use the appropriate command.
4. Set environment variables. Create a `.env.local` file and set the URL of the
Ayaka backend.

    ```env
    NEXT_PUBLIC_API_BASE_URL=http://localhost:9000
    ```

    Change the URL as needed.

## Start Development Server

To start the development server, run:

```bash
pnpm dev
```

or

```bash
npm run dev
```

```bash
yarn dev
```

The application will be available at `http://localhost:3000`.

## Project Structure

- `app/`: Page routing with Next.js App Router
  - `layout.tsx`: Root layout
  - `page.tsx`: Package list page
  - `about/page.tsx`: About Lumine page
  - `server-status/page.tsx`: Server status page
- `components/`: Reusable components
  - `ui/`: UI components from shadcn/ui
  - Other components (`package-table.tsx`, `search-bar.tsx`, etc.)
- `hooks/`: Custom hooks
- `lib/`: Utility functions and type definitions
  - `api.ts`: Backend API
  - `types.ts`: Type definitions
  - `utils.ts`: Other utilities
- `styles/`: Global styles

## License

See [LICENSE.txt](https://github.com/Hayao0819/Kamisato/blob/main/LICENSE.txt).
