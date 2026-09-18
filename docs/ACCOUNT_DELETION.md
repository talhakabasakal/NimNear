# Account deletion

NIMNear account deletion is anonymize-and-deactivate, not a hard `DELETE FROM users`.

This document describes the implemented technical behavior. It is not a legal
assessment and does not claim GDPR, KVKK, or similar compliance.

## Model

`DELETE /api/v1/me` is authenticated self-service deletion. The target is always
the current session user. The handler never accepts a `user_id` body field.

Deletion runs in one database transaction. If a required step fails, the
transaction rolls back.

The account is marked `status=inactive` with `deleted_at` set. Cookie sessions
are cleared. Subsequent authenticated API calls reject the user even if an old
Bearer JWT is still cryptographically valid.

Repeating deletion on an already deleted row is a no-op at the repository.
Repeating `DELETE /api/v1/me` with an old JWT is rejected because the account
is no longer active.

## What becomes anonymous or inaccessible

Removed or blanked:

- email
- password hash
- first name, last name, display name
- username
- bio
- avatar URL

Linked Nimiq identities are revoked (`revoked_at`). Outstanding auth challenges
for those addresses are consumed. The same wallet cannot recreate or resume the
deleted account.

Public profile reads return not found. The original PII is not returned from
`GET /api/v1/me` because deleted users cannot authenticate.

## What is retained, and why

These rows stay because they are integrity or payment evidence:

- `users.id` (anonymized row)
- `events` owned by the user (`organizer_id` stays)
- `event_participants` (free RSVP remains so attendance counts stay consistent)
- `event_purchases` including hashes and statuses
- paid/completed `payment_requests`
- `consumed_nimiq_transactions`

Pending payment requests created by the deleted user are cancelled and are no
longer payable. Submitted or paid requests are not rewritten.

## Events and calendars

Existing events are not cascade-deleted.

Future published public events organized by the user are cancelled so they are
not silently actionable under a deleted organizer. Past published events remain
for history, with an anonymized organizer identity.

Owned calendars are archived, not hard-deleted. Public discovery hides them.
Existing events keep their `calendar_id`. Archived calendars cannot receive new
events.

## Nimiq evidence

Deletion does not alter Nimiq address derivation, verifier behavior, or
consumed transaction replay protection.
