# Event operations

Public event writes for organizers use the product API. Operator takedown is a
CLI, not an admin dashboard.

## Commands

From `backend/`:

```
go run ./cmd/manage-event list
go run ./cmd/manage-event list -city "Example City"
go run ./cmd/manage-event get -id <uuid>
go run ./cmd/manage-event cancel -id <uuid>
```

`cancel` and `takedown` are the same command. They call the same domain
cancellation transition as organizer cancel. RSVP rows, purchases, hashes, and
consumed transaction evidence are preserved. There are no refunds and no
payment-state edits.

The CLI does not accept payment hashes, merchant addresses, or purchase IDs.
