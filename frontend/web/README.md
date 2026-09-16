# NIMNear frontend

The NIMNear frontend is a Next.js App Router application prepared for Vercel and Nimiq Mini App/WebView use.

## Development

```bash
npm install
npm run dev
```

Validation commands:

```bash
npm run lint
npm run build
```

User-facing product records are loaded from the NIMNear backend APIs. Static copy, navigation labels, design tokens, loading skeletons, neutral media fallbacks, and explicitly labeled editorial catalog content are not application records. See `../../docs/DATA_PROVENANCE.md` for the route-by-route contract.
