# Cursor Prompt — Migrate NIMNear Frontend from Vite to Next.js + shadcn/ui

You are working in the NIMNear monorepo.

Repository root:

```text
nimnear/
├── AGENTS.md
├── README.md
├── docs/
├── backend/
└── frontend/
    └── web/
```

Before making any change:

1. Read `AGENTS.md`.
2. Read the relevant files under `docs/`, especially:
   - `docs/ARCHITECTURE.md`
   - `docs/FRONTEND.md`
   - `docs/DESIGN_SYSTEM.md`
3. Inspect the existing `frontend/web` Vite project.
4. Do not touch backend code unless absolutely necessary.

## Goal

Migrate `frontend/web` from React + Vite to:

- Next.js
- App Router
- TypeScript
- Tailwind CSS
- shadcn/ui

Keep the project compatible with the Nimiq Mini App / WebView use case.

The frontend will be deployed to Vercel.

## Important Constraints

- Preserve the monorepo layout.
- `frontend/web` must remain the frontend path.
- Do not create a separate repository.
- Do not modify or remove `backend`.
- Do not modify the working MasterFabric backend.
- Do not invent product screens.
- Figma is the source of truth for UI/UX, but do not implement the final Figma screens as part of this migration task.
- This task is infrastructure/setup only.

## Existing Frontend Packages

Inspect what is currently installed before changing dependencies.

The project previously used or planned to use:

- `@nimiq/mini-app-sdk`
- `qrcode`
- `html5-qrcode`

Preserve packages that are still needed and compatible.

Do not remove them merely because they are not used yet.

## Migration Requirements

### 1. Replace Vite with Next.js

Convert `frontend/web` into a valid Next.js App Router project.

Remove obsolete Vite-specific files only when they are no longer needed.

Examples may include:

- `vite.config.ts`
- Vite-specific `index.html`
- Vite-specific source bootstrap files

Do not blindly delete files without first checking whether they contain useful project code.

### 2. App Router

Use an `app/` directory.

Create the minimum valid application structure, for example:

```text
frontend/web/
├── app/
│   ├── globals.css
│   ├── layout.tsx
│   └── page.tsx
├── components/
│   └── ui/
├── lib/
├── public/
├── components.json
├── next.config.*
├── package.json
├── tsconfig.json
└── ...
```

This is a target shape, not a command to create empty folders unnecessarily.

### 3. Tailwind CSS

Set up Tailwind using the version and conventions compatible with the installed/current Next.js setup.

Do not mix obsolete Tailwind configuration patterns with the installed version.

If the existing Vite project already uses Tailwind v4, preserve the correct v4 approach where compatible.

### 4. shadcn/ui

Initialize shadcn/ui correctly for the Next.js project.

Create `components.json`.

Use a sensible default setup, but do not treat the default visual theme as the final NIMNear design.

Add only a very small number of primitives required to verify setup.

For example, adding `Button` is enough.

Do not install a large set of unused shadcn components.

Shared shadcn primitives should live under:

```text
frontend/web/components/ui
```

unless the actual initialized configuration requires a clearly better equivalent.

### 5. Minimal Home Page

Create a minimal temporary home page only to prove the migration works.

It should not invent the NIMNear product UI.

A simple page such as:

```text
NIMNear
Frontend setup is ready.
```

with one shadcn Button is enough.

Do not design the actual discovery screen yet.

### 6. Client/Server Boundaries

Do not mark the entire app as `"use client"`.

Use Server Components by default.

Only use client components where necessary.

Nimiq SDK, geolocation, camera/QR scanning and browser APIs will later require client-side boundaries.

Do not implement those integrations yet unless required only to resolve a build incompatibility.

### 7. Environment Variables

Do not add secrets.

If a frontend backend-base-url variable is needed, use an appropriate Next.js convention.

Only values safe for the browser may use `NEXT_PUBLIC_*`.

Do not expose backend secrets.

### 8. Package Scripts

Make sure `package.json` has working scripts for at least:

```bash
npm run dev
npm run build
npm run lint
```

If a typecheck script is useful and the project already uses one, preserve or add it.

### 9. Oxlint

The existing frontend used Oxlint.

Inspect the current config before deciding what to do.

Prefer preserving Oxlint if it remains compatible with the Next.js project.

Do not add ESLint solely because older Next.js templates used it if it is not required by the installed Next.js version.

If Next.js tooling requires a change, explain the reason in the final report.

### 10. TypeScript

Keep strict TypeScript settings where practical.

Avoid `any`.

Ensure aliases such as `@/*` are configured consistently if shadcn initialization uses them.

### 11. Nimiq Compatibility

Inspect whether `@nimiq/mini-app-sdk`, `qrcode`, or `html5-qrcode` cause SSR/build issues.

Do not remove them without reason.

If a package is browser-only, do not import it from a Server Component.

Do not implement wallet/payment behavior yet.

### 12. Existing Code

Before deleting old Vite code:

- inspect it;
- preserve reusable assets, CSS, types, or components if they are meaningful;
- migrate useful code where appropriate.

If the existing frontend is essentially the default Vite starter, remove obsolete starter content cleanly.

### 13. Figma

Do not implement the full Figma design in this task.

The purpose of this migration is to make the frontend ready for a later Figma MCP driven implementation.

Do not invent:
- navigation;
- bottom bars;
- cards;
- colors;
- typography;
- screen flows.

### 14. Documentation

After the migration, verify that root documentation reflects Next.js + shadcn/ui.

Do not overwrite unrelated documentation.

If the repository already contains the updated Next.js documentation, leave it alone.

## Validation

Run from `frontend/web`:

```bash
npm install
npm run lint
npm run build
```

Also run any relevant typecheck command if configured.

Start the dev server and verify the home page loads successfully.

Default expected local frontend URL:

```text
http://localhost:3000
```

If port 3000 is occupied and another port is used automatically, report it.

## Git / Safety

Do not commit `.env.local` or any secrets.

Do not create a nested `.git`.

Do not reset unrelated user changes.

Do not use destructive Git commands.

## Final Report

When finished, report:

1. Which files were created.
2. Which files were removed.
3. Which files were modified.
4. Final frontend stack.
5. Next.js version.
6. Tailwind version.
7. shadcn/ui initialization result.
8. Whether Oxlint was preserved and how linting works.
9. Whether `@nimiq/mini-app-sdk` remains installed.
10. Whether `qrcode` and `html5-qrcode` remain installed.
11. `npm run lint` result.
12. `npm run build` result.
13. TypeScript/typecheck result if applicable.
14. Dev server result.
15. Local URL.
16. Any warnings or blockers.

Make the changes and validate them. Do not only explain what should be done.
