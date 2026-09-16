# NIMNear Web

This is the NIMNear Next.js 16 App Router frontend. It uses TypeScript,
Tailwind CSS v4, shadcn/ui conventions, Lucide icons, and
@nimiq/mini-app-sdk.

## Routes

    /
    /events
    /events?view=past
    /events/[id]
    /events/create
    /places/[id]
    /calendars
    /calendars/[id]
    /profile
    /profiles/[id]
    /profile/edit

Dynamic event, place, calendar, and profile records are fetched from the Go API.
The home category taxonomy and interface copy are local UI configuration, not
application records. No static event/profile/payment/location record is used
as an API fallback.

## Rendering and state

App Router pages are Server Components by default. Client Components are used
for browser-only behavior and local interaction:

- NimiqConnect for explicit native account connection;
- geolocation and nearby-place discovery;
- event RSVP/purchase interaction and bounded polling;
- profile editing and calendar workspace forms.

Each data route has a route-level loading boundary. API errors render explicit
error states, successful empty collections render distinct empty states, and
404 responses map to the route not-found UI where supported. A failed API
request never becomes a fabricated collection or success state.

## API configuration

The typed API layer resolves its base URL as:

1. NEXT_PUBLIC_NIMNEAR_API_URL for browser requests;
2. NIMNEAR_API_URL for server requests;
3. http://localhost:8080 as local fallback.

NEXT_PUBLIC_* values are exposed to the browser and must contain no backend
secret. A device/WebView cannot reach localhost on the developer machine; use a
reachable LAN/deployed URL and configure backend CORS for the real frontend
origin. Next development origin allowances are configured separately for the
local Mini App workflow.

## Nimiq boundary

Normal Mini App account entry is the shared NimiqConnect component. Only an
explicit user action calls:

    const nimiq = await init({ timeout: 10000 });
    const accounts = await nimiq.listAccounts();

Nimiq Pay owns the native approval UI. The frontend handles cancellation,
ErrorResponse, unavailable provider, and empty-account states and displays a
shortened returned NQ address when connection succeeds. It does not call
listAccounts() on page load, build a custom wallet modal, handle private keys,
or turn the result into a JWT.

The backend still expects the existing JWT for event creation, free RSVP,
profile editing, calendar mutations, and purchase operations. A native
connection without that JWT is shown as blocked where a protected action needs
a server session. The signature-to-JWT bridge is intentionally not implemented;
see docs/NIMIQ_AUTH_CONTRACT.md.

## Data/API rules

- Use the typed API modules under lib/api; do not scatter raw fetch calls.
- Treat backend null optional fields as nullable and tolerate older omitted
  fields without inventing values.
- Render money from backend integer Luna/price_nim semantics; never use
  floating-point wallet amounts or derive the payment amount from a label.
- Use backend event ordering and filters. The homepage section is
  Yaklaşan etkinlikler, not a popularity ranking.
- Render external media through the current validated browser img path. There is
  no approved host allowlist or first-party upload provider yet; do not widen
  remote image configuration casually.
- Place cards and city/location context appear only from real API responses
  after explicit geolocation where applicable. Do not restore a static Istanbul
  fallback.

## Development and validation

    npm install
    npm run dev
    npm run lint
    npx tsc --noEmit
    npm run build
    npm run test:a3

Figma is the visual source of truth. Keep the dark theme and confirmed tokens
in docs/DESIGN_SYSTEM.md; do not add shadows or a second icon library.
