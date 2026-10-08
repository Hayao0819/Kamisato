# Lumine Web

Lumine is the web console for Ayato: browse packages and signatures, publish
packages, submit builds, and inspect Miko jobs through Ayato. Optional operations
are shown only when Ayato advertises the corresponding feature.

## Development

Use the Node and pnpm versions declared in `package.json`:

```sh
cd lumine/web
corepack enable
pnpm install --frozen-lockfile
AYATO_URL=http://localhost:8080 pnpm dev
```

Open `http://localhost:3000`. Next.js proxies `/api` and `/repo` to `AYATO_URL`
(default `http://localhost:8080`) in development. The browser uses same-origin
cookie authentication; there is no browser-local server registry.

```sh
pnpm gen:types
pnpm typecheck
pnpm test
pnpm build
```

`pnpm build` creates a static export at `lumine/embed/out`. The Go Lumine server
embeds this directory and, in cookie mode, proxies Ayato on the browser's origin:

```sh
go run ./lumine --addr :3000 --ayato-url http://localhost:8080
```

Run that command from the repository root after building the web export.

## Runtime configuration and authentication

The browser reads `/env.json` at startup. An empty `AYATO_URL` means same-origin;
`AUTH_MODE` is `cookie` by default. The Go server writes this configuration from
`LUMINE_AYATO_URL` / `--ayato-url` and `LUMINE_AUTH_MODE` / `--auth-mode`.

For a fully static, cross-origin deployment, use `AUTH_MODE: "bearer"` and set
`AYATO_URL` to the public Ayato URL. The
[build-lumine action](../../actions/build-lumine/action.yml) injects the runtime
configuration and the API origin into the static host's `_headers` CSP. Ayato
must allow the frontend's origin for CORS and its web login callback.

Cookie login uses a first-party HttpOnly session cookie. Bearer login keeps the
token in memory, not browser storage. `APIClient` applies this auth strategy to
requests. Build-log streams use Ayato's short-lived, one-time job token because
native `EventSource` cannot attach an Authorization header. Do not place a
long-lived bearer token in a stream URL.

`TITLE` and `DESCRIPTION` in `/env.json` override the landing text; the Go server
also accepts `LUMINE_TITLE` and `LUMINE_DESCRIPTION`.

## Source ownership

- `src/app/`: Next.js route entries and route-specific clients. `/` is the search
  landing page; `/packages` is the query-driven package list.
- `src/components/`: Console views and shared presentation; `ui/` contains the
  Radix/shadcn primitives. Keep page-only behavior in its route rather than
  inventing a parallel application or service layer.
- `src/hooks/`: React state and effects, including shared console atoms.
- `src/lib/`: Framework-independent URL/query policies, API/auth clients, type
  contracts, and formatting. Pure query code does not import React hooks.
- `src/styles/globals.css`: Tailwind CSS v4 theme and styles.

API calls and auth delivery belong to `src/lib/api.ts` and `auth-client.ts`, not
individual views. The `/packages` URL owns scope, filters, sorting, and pagination;
related query fields must be updated in one navigation.

## Generated API types

`src/lib/generated/` is generated from Ayato's domain responses and Miko's public
client contracts using `tygo.yaml`. `src/lib/types.ts` contains only client-side
refinements. Regenerate when the Go contracts change:

```sh
pnpm gen:types
```

The generator version is pinned in the root `go.mod`; `go generate ./lumine`
uses the same tool. Do not hand-edit generated files. CI checks regeneration,
TypeScript, and the unit tests.

## License

See [LICENSE.txt](../../LICENSE.txt).
