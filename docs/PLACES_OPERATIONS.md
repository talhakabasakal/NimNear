# Place operations

Places are publicly readable NIMNear inventory. Writes are operator-managed.
There is no public user place CRUD.

Duplicate names are allowed. The UUID primary key is the authoritative identity.
Operators should list existing matches before creating a place with a familiar
name. P0 does not add a global unique name constraint or fuzzy matching.

## Commands

From `backend/`:

```
go run ./cmd/manage-place create \
  -name "Example Harbor Cafe" \
  -category cafe \
  -description "Fictional development cafe" \
  -address "1 Example Harbor Walk" \
  -lat 41.0400 \
  -lon 28.9900 \
  -image-url https://example.com/media/harbor-cafe.jpg \
  -active=true

go run ./cmd/manage-place update -id <uuid> -name "Example Harbor Cafe" -lat 41.0410 -lon 28.9910
go run ./cmd/manage-place disable -id <uuid>
go run ./cmd/manage-place list
go run ./cmd/manage-place list -name "Example Harbor Cafe"
```

`create` prints any existing places that share the same name, then still
creates the new record.

## Required fields

| Field | Rule |
| --- | --- |
| name | Required, 1–255 characters after trim |
| latitude | Required, finite number in `[-90, 90]` |
| longitude | Required, finite number in `[-180, 180]` |
| description | Optional, max 5000 characters |
| address | Optional, max 500 characters |
| category | Optional, max 100 characters, `^[A-Za-z][A-Za-z0-9_-]*$` |
| image_url | Empty or an absolute HTTPS URL, max 2048 characters, no credentials |
| is_active | Defaults to true on create |

NaN and Inf coordinates are rejected. HTTP, `javascript:`, `data:`, and other
non-HTTPS image URLs are rejected.

## Coordinates

Use decimal degrees. Example: `-lat 41.0400 -lon 28.9900`. Nearby search uses
the same WGS84 pair through `GET /api/v1/places/nearby?lat=...&lng=...`.

## Development seed

Fictional places only. Never hardcode real businesses. Never runs at startup,
in migrations, or in production.

```
make seed-dev-places
```

The command is idempotent and uses deterministic UUIDs. It refuses
`APP_ENV=production`.

After seeding, verify:

```
curl "http://localhost:8080/api/v1/places/nearby?lat=41.0400&lng=28.9900"
curl "http://localhost:8080/api/v1/places/11111111-1111-4111-8111-111111111111"
curl "http://localhost:8080/api/v1/events?place_id=11111111-1111-4111-8111-111111111111"
```

Disabled places are omitted from nearby results and return 404 from public
detail. Operator `GetByID` still sees them.
