# NIMNear — Event Lifecycle and Participation Truth

## Scope

P1A defines the event lifecycle and the single product-facing attendance model. Nimiq verification, recovery, recipient checks, Luna conversion, and payment semantics remain unchanged.

## Participation truth

A public attendee is one distinct user in the union of:

- `event_participants` (free RSVP)
- `event_purchases` with `status = confirmed` (paid participation)

The same user counts once. Pending, submitted, verifying, failed, expired, and cancelled purchases are not public attendees.

`events.attendee_count` is synchronized from this union when RSVP or paid confirmation changes. Read projections also derive the union so public counts do not rely on an unverified client value.

## Capacity truth

Admission occupancy is broader than public attendance:

- free RSVP;
- confirmed paid purchases;
- submitted/verifying purchases with an active verification flow;
- unexpired pending paid holds.

All occupancy queries use a distinct user union. A pending or verifying payer may reserve the last seat without appearing in `attendee_count`. When a pending hold expires, capacity is released; the public attendee count never decrements for a hold that was never public attendance.

RSVP and purchase creation lock the event row in PostgreSQL. Capacity edits cannot reduce capacity below this authoritative occupancy.

## Event lifecycle

Creation remains immediate publication. P1A adds organizer-only mutations:

- `PATCH /api/v1/events/{id}`
- `POST /api/v1/events/{id}/cancel`

The organizer is derived from the authenticated session. Unknown PATCH fields are rejected. Editable fields are title, description, schedule, image URL, capacity, and place association. Price, payment state, recipient, counters, IDs, and transaction data are not editable.

Schedule edits after the event starts are rejected. Capacity reductions below occupancy are rejected. Location changes after active or confirmed paid participation are rejected. Cancellation is a state transition, not deletion.

Cancellation preserves the event, RSVP rows, purchase rows, and transaction evidence. New RSVP and purchase creation are unavailable because normal discovery excludes cancelled events. Direct detail remains available and clearly shows `cancelled`. Refunds are not implemented and must not be implied by the UI.

## Profile history

Attended profile lists and counts include free RSVP and confirmed paid events, with one event returned once. Failed, expired, cancelled, pending, submitted, and verifying purchases are excluded.

## Nimiq boundary

Do not modify `NimiqVerifier`, sender binding, recipient verification, network/finality checks, consumed transaction protection, recovery semantics, or Mini App transaction behavior as part of event lifecycle work.
