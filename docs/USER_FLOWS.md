# NIMNear — User Flows

## Status

The flows below are limited to screens and navigation visibly confirmed in the connected NIMNear Figma Make file.

Do not infer missing screens or hidden transitions from this document.

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

The `Keşfet` screen exposes city cards that lead to the confirmed İstanbul city detail page. A footer also exposes `Keşfet`, `Fiyatlandırma`, `Yardım`, and `Uygulamayı İndir` controls.

## Keşfet

Confirmed visible hierarchy:

1. Istanbul panorama hero with nearby context and event summary.
2. Popular events section with event cards and a `Tümünü Görüntüle` action.
3. Category grid.
4. Featured community calendars with `Takip et` actions.
5. Regional tabs.
6. City cards and a larger city list.

Selecting a city card leads to a city detail page. Event-card detail navigation was not confirmed.

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

The result of creating a calendar was not inspected.

## Etkinlik oluştur

Confirmed visible controls include:

- Event name input.
- Separate landscape selector: `Yok`, `Gece Gökyüzü`, `Şehir`, `Okyanus`, `Orman`, `Çöl`, `Çiçek`.
- Theme selector: `Mor`, `Gece`, `Okyanus`, `Orman`, `Amber`, `Gül`.
- Personal/public calendar options.
- Start and end date/time controls.
- Location and description controls.
- Ticket price, approval, and capacity controls.
- `Etkinlik oluştur` submit action.

The post-submit flow was not inspected.

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

Location behavior was not confirmed by the inspected Figma screens. Do not invent permission, denial, unavailable-location, or fallback transitions from this document.

## Unconfirmed Flows

The following must remain explicitly unconfirmed until the corresponding Figma states are inspected:

- checkout;
- visual checkout frame (not inspected in Figma);
- visual payment-processing/success frames (not inspected in Figma);
- ticket display;
- QR code display;
- calendar creation result;
- notification behavior;
- location permission and fallback behavior.

Do not add routes, tabs, or transitions for these flows based only on the mention in Figma Make history.


## Event detail participation

The implemented event detail page supports the following backend-confirmed behavior:

- Free upcoming events expose Katıl for authenticated users.
- Anonymous users use the existing login/register flow before participation.
- An attending user sees the attending state and can choose Katılımı iptal et.
- Capacity-limited events show the current count and a disabled Tükendi state when full.
- Past events do not expose an RSVP action.
- Paid upcoming events show the price and an authenticated Satın al action. The action opens Nimiq Pay, submits the returned transaction hash, and shows submitted/verifying until backend finality confirms the purchase.

Participation is user-specific and does not imply invitations, payments, tickets, QR codes, calendars, notifications, or waitlists.



## Implemented Profile read flow

The confirmed Profile screen is now connected to the first backend profile read model:

1. App header profile control opens /profile.
2. Anonymous users see the existing authentication panel.
3. An authenticated user sees the real display name, optional profile fields, join date, organized/attended counts, and event history.
4. Public profiles are available at /profiles/[id] when linked from an event organizer block.
5. Organized and attended lists use empty and API error states when no records or data are available.

Profile editing is available only to the authenticated owner at `/profile/edit`. The inferred flow edits display name, username, and bio, then returns to `/profile` after a successful PATCH. Public profiles remain read-only. Email and authentication metadata are not part of public profile presentation.

## Paid-event payment flow

The implemented paid-event flow stops after confirmed purchase:

1. An upcoming paid event shows Satın al; past and sold-out events do not offer a purchase action.
2. An anonymous user sees the existing login/register panel.
3. An authenticated user creates or reuses a server-owned pending purchase.
4. The frontend retrieves backend-authoritative recipient, exact Luna amount, and network.
5. Nimiq Pay requests user confirmation through sendBasicTransaction.
6. Wallet rejection is a normal retryable state and leaves the purchase pending.
7. The wallet hash is submitted to the authenticated transaction endpoint.
8. The event detail shows İşlem ağa gönderildi / Ödeme doğrulanıyor while the backend checks RPC inclusion and macro finality.
9. Only a server-confirmed finalized transfer shows Ödeme doğrulandı.
10. Reopening the event recovers the persisted purchase state.

This is an implemented product/API flow, but the inspected Figma file did not confirm dedicated checkout, processing, or payment-success frames. Ticket, QR, check-in, refund, payout, notification, and multiple-ticket flows remain unconfirmed and unimplemented.
