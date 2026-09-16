# NIMNear — User Flows

## Status

The flows below distinguish Figma-confirmed visual references from behavior confirmed in the current repository.

Do not infer missing screens or hidden transitions from this document. The Istanbul/city composition is a Figma visual reference; the runtime does not assume Istanbul as a location fallback.

## Confirmed Screens

The following screens are confirmed in Figma:

- `Keşfet`
- `Etkinlikler`
- `Takvimler`
- `Etkinlik oluştur`
- `Profil`
- İstanbul city detail page

Checkout, processing, and success frames were mentioned in Figma Make history but were not inspected as visible screens. The payment API flow is implemented separately; the visual frames remain unconfirmed. Ticket and QR states remain unconfirmed.

## Navigation

The confirmed shared navigation contains:

- `Etkinlikler`
- `Takvimler`
- `Keşfet`
- `Etkinlik oluştur`
- Profile/avatar access
- Theme toggle and notifications controls

The `Keşfet` screen exposes location-driven place cards after the user explicitly shares location. Each real place card leads to `/places/{id}`. The Figma İstanbul city-detail composition remains a visual reference; a city taxonomy/subscription surface is not implemented. A footer also exposes `Keşfet`, `Fiyatlandırma`, `Yardım`, and `Uygulamayı İndir` controls.

## Keşfet

Confirmed visible hierarchy:

1. Istanbul panorama hero with nearby context and event summary.
2. Upcoming events section with event cards and a `Tümünü Görüntüle` action.
3. Category grid.
4. Featured community calendars with `Takip et` actions.
5. Regional tabs.
6. City/place discovery cards and a larger city list.

The implemented discovery path uses backend active places and stable place IDs after explicit location permission. Event-card detail navigation was not confirmed.

## Etkinlikler

Confirmed visible hierarchy:

1. Istanbul image strip and `Etkinlikler` heading.
2. `Yaklaşan` and `Geçmiş` controls.
3. Date-grouped event timeline.
4. Event cards showing time, title, organizer, location, availability/status, price, attendance count, and image.

The visible event states include `Davetli`, `Ücretsiz`, `Tükendi`, and NIM-priced events. No event detail or checkout transition was confirmed.

## Takvimler

Confirmed visible hierarchy:

1. Welcome/onboarding card with `İleri`.
2. `Takvimlerim` section with `Oluştur` and an empty state.
3. `Takip edilenler` section with an empty state.

The implemented `/calendars` route uses real public calendar records, with explicit loading, empty, error, and not-found states. `/calendars/[id]` reads the active public calendar and its published/public associated events. `Takvimlerim` and `Takip edilenler` use the existing JWT-protected `/api/v1/me/calendars` endpoint. Calendar creation and follow/unfollow are protected by the legacy JWT; a native Nimiq connection without a verified backend session is clearly blocked.

## Etkinlik oluştur

Figma-confirmed controls include event name, landscape/theme, calendar visibility,
start/end date-time, location, description, ticket price, approval, capacity,
and the `Etkinlik oluştur` action.

The current supported implementation persists event name, description,
start/end timestamps, city, optional image URL, optional owned calendar,
optional active place or custom address/coordinates, optional capacity, and
exact free/paid `price_nim`. Landscape, theme, calendar visibility, and
approval remain omitted until the backend has a persisted contract for them.

The current supported create flow is legacy-JWT protected. It validates the returned event ID and navigates to `/events/{id}` only after `POST /api/v1/events` succeeds. The supported persisted controls are title, description, start/end timestamps, city, optional image URL, optional owned calendar, optional active place or custom address/coordinates, optional positive capacity, and free/paid `price_nim` with exact Luna conversion. Figma-only landscape, theme, and approval controls are intentionally not rendered because they are not persisted by the current backend.

## İstanbul City Detail

Confirmed visible hierarchy:

1. Full-width Istanbul/Galata hero image.
2. City title, time, description, and `Abone ol` action.
3. Date-grouped event timeline.
4. City information card and aerial image.
5. Neighborhood labels including Kadıköy, Galata, Nişantaşı, Karaköy, and Beşiktaş.

The behavior of subscription and event selection beyond the visible page was not confirmed.

## Profil

Confirmed visible hierarchy:

1. Avatar, name, handle, bio, and join date.
2. Organized/attended statistics.
3. Empty public-event state.
4. `İlk etkinliğini oluştur` action.

The existing profile read flow is Figma-confirmed. Profile editing is implemented as a restrained inferred flow because no matching edit frame was confirmed in the inspected Figma Make file.

## Location Flow

The implemented location flow is explicit and does not claim an implicit city:

1. User chooses `Konumumu kullan`.
2. Browser/WebView geolocation permission is requested.
3. Granted coordinates are used only for the bounded nearby-place request.
4. Denied, unsupported, timeout, backend-error, and no-nearby-place states are shown distinctly.
5. User can continue browsing upcoming events without sharing location.

Precise location is not stored as profile data and background tracking is not used.

## Unconfirmed Flows

The following must remain explicitly unconfirmed until the corresponding Figma states are inspected:

- checkout;
- visual checkout frame (not inspected in Figma);
- visual payment-processing/success frames (not inspected in Figma);
- ticket display;
- QR code display;
- notification behavior;
- Figma-specific city subscription and neighborhood behavior.

Do not add routes, tabs, or transitions for these flows based only on the mention in Figma Make history.


## Event detail participation

The implemented event detail page supports the following backend-confirmed behavior:

- Free upcoming events expose Katıl for authenticated users.
- Users with an existing backend JWT can participate. A native Nimiq connection without that JWT is shown as a blocked protected action; listAccounts() is not treated as authentication.
- An attending user sees the attending state and can choose Katılımı iptal et.
- Capacity-limited events show the current count and a disabled Tükendi state when full.
- Past events do not expose an RSVP action.
- Paid upcoming events show the price and an authenticated Satın al action. The action opens Nimiq Pay, submits the returned transaction hash, and shows submitted/verifying until backend finality confirms the purchase.

Participation is user-specific and does not imply invitations, payments, tickets, QR codes, notifications, or waitlists. Calendar ownership/follow state is a separate backend domain.



## Implemented Profile read flow

The confirmed Profile screen is now connected to the first backend profile read model:

1. App header profile control opens /profile.
2. Users without a legacy backend JWT see native Nimiq connection plus an explicit protected-action limitation.
3. An authenticated user sees the real display name, optional profile fields, join date, organized/attended counts, and event history.
4. Public profiles are available at /profiles/[id] when linked from an event organizer block.
5. Organized and attended lists use empty and API error states when no records or data are available.

Profile editing is available only to the authenticated owner at `/profile/edit`. The inferred flow edits display name, username, and bio, then returns to `/profile` after a successful PATCH. Public profiles remain read-only. Email and authentication metadata are not part of public profile presentation.

## Paid-event payment flow

The implemented paid-event flow stops after confirmed purchase:

1. An upcoming paid event shows Satın al; past and sold-out events do not offer a purchase action.
2. A native-only user can connect Nimiq Pay, but the protected purchase remains unavailable until the verified Nimiq-to-JWT bridge exists; an existing backend JWT is required.
3. An authenticated user creates or reuses a server-owned pending purchase.
4. The frontend retrieves backend-authoritative recipient, exact Luna amount, and network.
5. Nimiq Pay requests user confirmation through sendBasicTransaction.
6. Wallet rejection is a normal retryable state and leaves the purchase pending.
7. The wallet hash is submitted to the authenticated transaction endpoint.
8. The event detail shows İşlem ağa gönderildi / Ödeme doğrulanıyor while the backend checks RPC inclusion and macro finality.
9. Only a server-confirmed finalized transfer shows Ödeme doğrulandı.
10. Reopening the event recovers the persisted purchase state.
11. If the backend cannot conclusively verify the transfer before the configured reconciliation deadline, the purchase becomes expired and the UI does not show payment success or a ticket.

This is an implemented product/API flow, but the inspected Figma file did not confirm dedicated checkout, processing, or payment-success frames. Ticket, QR, check-in, refund, payout, notification, and multiple-ticket flows remain unconfirmed and unimplemented.
