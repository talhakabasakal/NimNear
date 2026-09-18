# NIMNear Nimiq Authentication Contract Investigation

Status: **implemented for Testnet** (2026-09-17).

The repository now implements the wallet-bound, single-use `AUTH_LOGIN`
challenge for both Nimiq Pay Mini Apps and the public Nimiq Testnet Hub. The
older investigation below is retained as design history. Its `NO-GO` verdict
was superseded by the current official Mini App provider surface, the installed
SDK types, the official Hub signed-message contract, and integration tests.

The executable contract and deployment settings are summarized in
`NIMIQ_AUTH_IMPLEMENTATION.md`.

## Sources inspected

Project and installed material:

- `AGENTS.md`
- `docs/FINALIZATION_AUDIT.md`
- `docs/ARCHITECTURE.md`
- `docs/BACKEND.md`
- `docs/FRONTEND.md`
- `.agents/skills/mini-apps/SKILL.md`
- All installed Mini App references: `references/nimiq-provider-api.md`,
  `references/checklist.md`, `references/scaffold.md`, `references/convert.md`,
  `references/chains-and-tokens.md`, and `references/ethereum-provider-api.md`.
- Installed package `frontend/web/node_modules/@nimiq/mini-app-sdk` version
  `0.1.0`, especially `dist/provider.d.ts`, `dist/provider.js`, and `README.md`.
- Existing auth and schema files, including
  `backend/internal/infrastructure/postgres/migrations/00002_create_users.sql`,
  the IAM model/DTOs, JWT service, router, and current profile/event code.

Official Nimiq material:

- [Nimiq Provider API](https://nimiq.dev/mini-apps/api-reference/nimiq-provider)
- [Official Mini App SDK source](https://github.com/nimiq/trust-web3-provider/tree/nimiq/packages/mini-app-sdk)
  at commit
  [`182fc351cd28620b6a186ceaa17ca60ef60318a0`](https://github.com/nimiq/trust-web3-provider/tree/182fc351cd28620b6a186ceaa17ca60ef60318a0/packages/mini-app-sdk)
- [Nimiq Web Client `PublicKey` reference](https://nimiq.dev/web-client/reference/classes/publickey)
- [Nimiq Web Client `Signature` reference](https://nimiq.dev/web-client/reference/classes/signature)
- Official core source at commit
  [`ea372ecbc7847e2b2b29aec252afbc374c8662c8`](https://github.com/nimiq/core-rs-albatross/tree/ea372ecbc7847e2b2b29aec252afbc374c8662c8):
  `keys/src/address.rs`, `keys/src/public_key.rs`,
  `keys/src/signature.rs`, `web-client/src/primitives/public_key.rs`, and
  `web-client/src/primitives/signature.rs`.
- [Official Hub signed-message guide](https://www.nimiq.dev/hub/guide/transactions),
  inspected only as a comparison. It is not treated as the Mini App contract.

## 1. Confirmed facts

### 1.1 Mini App provider surface

The official Mini App API documents:

```ts
const nimiq = await init()
const result = await nimiq.sign(message)
```

The installed SDK types define the exact application-facing result union as:

```ts
interface SignatureResult {
  publicKey: string
  signature: string
}

interface ErrorResponse {
  error: {
    type: string
    message: string
  }
}

sign(message: string | {
  message: string
  isHex?: boolean
}): Promise<SignatureResult | ErrorResponse>
```

The official Mini App API documents both returned fields as hex strings and
documents `PermissionDeniedError` when the user rejects the native
confirmation. The SDK implementation only forwards:

```ts
{ method: 'sign', params: typeof message === 'string'
  ? { message }
  : message }
```

It does not decode the message, hash it, add a prefix, verify the result, or
select a signer. A provider response may therefore be an `ErrorResponse` or a
rejected promise; both must be handled as authentication failure.

`listAccounts()` returns user-friendly NQ addresses. It is permission to read
accounts, not proof to the backend that the caller controls any address.

The installed SDK's `NimiqProvider.NETWORK` / `getNetwork()` value is the
provider namespace (`"nimiq"`). It is not documented as the active Nimiq
consensus network and must not be used as a mainnet/testnet assertion.

### 1.2 Official Nimiq key and signature primitives

The official Nimiq Web Client and core source establish that:

- Nimiq's ordinary public-key signature primitive is Ed25519.
- A raw Ed25519 public key is exactly 32 bytes.
- A raw Ed25519 signature is exactly 64 bytes.
- Verification is Ed25519 verification over a byte slice supplied to the
  verifier.
- The official core rejects an all-zero public key as a signer.
- The official core parses the Mini App-style hex representations through
  `PublicKey.fromHex` / `Signature.fromHex` equivalents.

These are properties of the official Nimiq core primitives. They do not, by
themselves, specify which bytes the Mini App host passes to Ed25519.

### 1.3 Public key to address derivation

The official core source establishes the ordinary single-signer derivation:

1. Decode `publicKey` from canonical hex into 32 raw bytes.
2. Compute the default Nimiq Blake2b digest of those public-key bytes.
3. Take the first 20 bytes of the 32-byte digest as the raw address.
4. Encode those 20 bytes as a user-friendly NQ address using Nimiq's address
   base32 alphabet and IBAN checksum formatting.

This is represented by the official `PublicKey.toAddress()` method and by
`impl From<&Ed25519PublicKey> for Address` in `keys/src/address.rs`.

The address derivation has no consensus-network parameter in the cited core
implementation. The authentication protocol should nevertheless bind the
challenge to the deployment's intended Nimiq network so a proof cannot be
reused across NIMNear environments.

### 1.4 TestAlbatross/MainAlbatross

The inspected core key, signature, and address code is independent of the
consensus network configuration. No TestAlbatross/MainAlbatross branch is
selected by the address or Ed25519 verification primitives themselves.

That does not prove that the Mini App host is connected to the backend's
configured network. The installed Mini App API exposes no documented method
that returns the active consensus network. Network selection must therefore be
an operational/configuration invariant until Nimiq documents a host-network
query for Mini Apps.

## 2. Exact Mini App `sign()` contract: established and missing

### Established

- Input is a string or `{ message: string, isHex?: boolean }`.
- The SDK forwards the value as the `sign` provider request.
- The result shape is `{ publicKey, signature }`.
- Official Mini App documentation says both result fields are hex strings.
- Native user confirmation is required.
- Cancellation is documented as `PermissionDeniedError`; the installed type
  also permits an `ErrorResponse` result.

### Not established by the Mini App sources

The inspected Mini App documentation, SDK source, SDK types, and public host
adapter source do not specify:

- whether a plain string is encoded as UTF-8 bytes exactly;
- what `isHex: true` does to the input;
- whether the message is prefixed with `Nimiq Signed Message`;
- whether the message is hashed before Ed25519 signing;
- which hash, if any, is used;
- whether message length is measured in Unicode code points, UTF-8 bytes, or
  another unit;
- whether hex output is lower-case and whether leading zeroes are preserved;
- whether `sign()` signs the currently active account, asks the native UI to
  select an account, or uses another host-defined signer rule;
- whether an address can be supplied to `sign()` to select or constrain the
  signer. The installed type has no address/signer parameter.

The official Hub guide describes a Keyguard/Hub-specific signed-message
format involving a Nimiq prefix and SHA-256. That guide is not evidence that
the Mini App provider uses the same transform. Applying that prefix in NIMNear
without Mini App confirmation would be cryptographic guessing.

## 3. Unresolved facts that block implementation

### P0: signed bytes

NIMNear cannot safely implement backend verification until Nimiq provides one
of the following for Mini App `sign()`:

1. an official statement that the signed preimage is raw UTF-8 bytes of the
   supplied message;
2. an official statement of the exact prefix, length encoding, hashing
   algorithm, and byte encoding; or
3. an official host/core source path showing the Mini App sign implementation
   and its test vectors.

The Hub format must not be assumed to be the Mini App format.

### P0: signer selection with multiple accounts

The real device returned two accounts. The Mini App method accepts only a
message, not an address or signer index. The official Mini App API does not
state which account signs when multiple accounts are approved, and the public
SDK is only a request forwarder.

The returned public key can identify the signer after signing, because it can
be converted to an address. That detects a mismatch with a challenge's claimed
address, but it does not tell the provider to sign a particular address. A
frontend-selected address cannot be treated as proof.

NIMNear needs official confirmation of one of these behaviors:

- Pay's native signing UI selects and displays the signer and returns its
  public key;
- Pay signs with a documented active account selected by the native account
  context; or
- the Mini App provider gains a documented signer/address parameter.

Until then, authentication for a two-account approval cannot be defined
without inventing a custom security mechanism.

### P1: Go implementation support

No official Nimiq Go verification library was found in the inspected official
Nimiq repositories, installed dependencies, or this repository. The official
implementation is Rust with a Web Client WASM/TypeScript surface.

After the Mini App contract is confirmed, a Go implementation can use
`crypto/ed25519` for the signature primitive and a vetted Blake2b package for
address derivation, backed by official Nimiq golden vectors. That is not safe
to implement until the signed preimage and signer-selection contract are
confirmed. A JavaScript verification service is not recommended.

## 4. Proposed NIMNear challenge format

This is a proposed NIMNear protocol format, not a claim that Nimiq will sign
it with any particular prefix or digest.

The backend generates and stores the exact message. The frontend must pass the
returned `message` verbatim to `nimiq.sign(message)`. It must not reconstruct,
trim, normalize, localize, or reserialize it.

Proposed canonical message, encoded as UTF-8 text with LF line endings and no
trailing newline:

```text
NIMNear Nimiq authentication
version: 1
domain: <NIMNEAR_AUTH_DOMAIN>
audience: nimnear-api
network: <NIMNEAR_NIMIQ_NETWORK>
address: <canonical-user-friendly-NQ-address>
challenge: <base64url-encoded-32-random-bytes>
issued_at: <RFC3339 UTC timestamp>
expires_at: <RFC3339 UTC timestamp>
```

Rules:

- `domain` is a backend-configured canonical origin/application identifier,
  not a frontend-provided value.
- `audience` is a fixed service identifier.
- `network` is a backend-configured value such as `test-albatross` or
  `main-albatross`, not `NimiqProvider.getNetwork()`.
- `address` is a claim used to bind the challenge. It is not trusted until the
  backend derives the same address from the returned public key.
- `challenge` is generated with a cryptographically secure random source and
  is at least 32 bytes before base64url encoding.
- `issued_at` and `expires_at` are server timestamps. The target lifetime is
  five minutes or less.
- `version` permits a future signed-message contract or canonical-format
  revision without ambiguity.
- The frontend uses the string form, not `{ isHex: true }`, unless Nimiq
  documents a different required mode for authentication messages.
- NIMNear must not add a Nimiq prefix or hash before calling `sign()` unless
  Nimiq explicitly defines that as the Mini App contract.

Because the provider's exact signed-byte transform is unresolved, the
verification function below intentionally contains a protocol placeholder.
It must be replaced by the official Mini App rule before implementation.

## 5. Proposed API endpoints

### `POST /api/v1/auth/nimiq/challenges`

Unauthenticated. Request:

```json
{
  "network": "test-albatross",
  "address": "NQ..."
}
```

The backend validates that the claimed address is a syntactically valid
canonical NQ address and that the requested network equals the server's
configured network. It does not treat the address as authenticated.

Response:

```json
{
  "challenge_id": "uuid",
  "message": "the exact stored message",
  "network": "test-albatross",
  "address": "NQ...",
  "issued_at": "RFC3339 UTC",
  "expires_at": "RFC3339 UTC"
}
```

The server stores the exact message bytes or exact canonical string. A
challenge should be short-lived and rate-limited.

### `POST /api/v1/auth/nimiq/verify`

Unauthenticated. Request:

```json
{
  "challenge_id": "uuid",
  "message": "the exact message returned by the challenge endpoint",
  "public_key": "hex",
  "signature": "hex"
}
```

The request does not need an address field. The backend derives the address
from the public key and compares it to the server-stored challenge address.
If the API retains an address field for diagnostics, it must be ignored for
trust decisions and checked against the derived address.

Success returns the existing session shape (`token` plus a safe user DTO),
after resolving or creating the internal `users.id`. The response must not
expose password hashes, private keys, or challenge internals.

Failure responses should be stable application errors, for example:

- `401 invalid_nimiq_signature`
- `401 public_key_address_mismatch`
- `400 challenge_message_mismatch`
- `400 unsupported_nimiq_network`
- `410 challenge_expired`
- `409 challenge_already_used`
- `429 authentication_rate_limited`

No SQL, parser, or cryptographic library errors should be returned directly.

## 6. Proposed database schema

This is a future migration design only; no migration is being added now.

### `auth_challenges`

Minimum fields:

```text
id                    UUID primary key
nonce                 BYTEA not null unique
version               SMALLINT not null
domain                TEXT not null
audience              TEXT not null
network               TEXT not null
claimed_address       TEXT not null
message               TEXT or BYTEA not null
issued_at             TIMESTAMPTZ not null
expires_at            TIMESTAMPTZ not null
consumed_at           TIMESTAMPTZ null
created_at            TIMESTAMPTZ not null
```

The nonce must be unpredictable and unique. Store the exact signed message so
verification is against server-issued bytes rather than a client reconstruction.

### `user_nimiq_identities`

Minimum fields:

```text
id                    UUID primary key
user_id               UUID not null references users(id)
network               TEXT not null
address               TEXT not null
public_key            BYTEA not null
verified_at           TIMESTAMPTZ not null
last_verified_at      TIMESTAMPTZ not null
revoked_at            TIMESTAMPTZ null
created_at            TIMESTAMPTZ not null
```

Required constraints:

- unique `(network, address)`;
- unique `(network, public_key)`;
- only non-revoked identities may be used as organizer recipients;
- no private key, seed phrase, device identifier, or wallet secret is stored.

The address may be stored in canonical user-friendly form for display, while
the server may additionally store its 20 raw bytes if that simplifies exact
comparison. The public key must be stored in canonical raw 32-byte form or a
canonical lower-case hex representation, consistently.

### Existing `users` compatibility issue

`00002_create_users.sql` currently requires a non-null unique `email`, while
Nimiq-native registration has no email. A native-only user cannot be created
safely without one of these future changes:

1. make `email` nullable and keep legacy email/password users unchanged;
2. move legacy credentials into a separate credentials table; or
3. require a native user to link an existing legacy account first.

Do not generate an email from an address, name, or device identifier. The
recommended path is nullable legacy credentials plus the identity table, with
the existing JWT subject remaining the internal `users.id`.

## 7. Verification pseudocode

The following describes the intended trust boundary. `SignedPreimage` is
deliberately unresolved and must be filled only from an official Mini App
contract.

```text
challenge = load challenge by challenge_id
if challenge is missing: reject
if challenge.consumed_at is not null: reject
if now >= challenge.expires_at: reject
if request.message != challenge.message: reject
if challenge.network != server.configured_nimiq_network: reject
if challenge.domain != server.configured_auth_domain: reject
if challenge.audience != "nimnear-api": reject

publicKeyBytes = strict_lower_or_case_insensitive_hex_decode(request.public_key)
signatureBytes = strict_lower_or_case_insensitive_hex_decode(request.signature)
if len(publicKeyBytes) != 32: reject
if len(signatureBytes) != 64: reject

derivedAddress = NimiqAddressFromPublicKey(publicKeyBytes)
if CanonicalNQ(derivedAddress) != CanonicalNQ(challenge.claimed_address): reject

signedBytes = OfficialMiniAppSignPreimage(challenge.message)
if !Ed25519Verify(publicKeyBytes, signedBytes, signatureBytes): reject

atomically:
  UPDATE auth_challenges
  SET consumed_at = now
  WHERE id = challenge.id
    AND consumed_at IS NULL
    AND expires_at > now
if affected_rows != 1: reject as replay/race

identity = find active identity by (challenge.network, derivedAddress)
if identity exists and public_key != publicKeyBytes: reject or require explicit
  identity recovery/linking policy; never silently replace the key
if identity does not exist:
  resolve/create internal users.id under a product-approved policy
  insert identity with unique constraints

issue existing NIMNear JWT with subject = internal users.id
```

Verification must reject malformed key/signature encodings before calling the
cryptographic library. The official Nimiq core's all-zero-public-key rejection
should be mirrored explicitly or delegated to a verified implementation.

## 8. Replay prevention and abuse controls

- Generate the nonce only on the backend using a cryptographically secure RNG.
- Store the nonce/challenge with a unique constraint.
- Use a short expiration, target five minutes or less.
- Store the exact message and bind it to domain, audience, network, address,
  version, issue time, and expiry.
- Consume with a conditional database update so concurrent verification can
  issue at most one session.
- Never accept a client-generated challenge or reconstructed message.
- Rate-limit challenge creation and verification by IP/origin and, after a
  verified identity exists, by identity.
- Do not use the Mini App device identifier as authentication; the installed
  SDK documents it as stable across different accounts on one device.
- Do not log full signatures, public keys, challenge messages, or addresses
  unnecessarily. If security telemetry needs identifiers, use an approved
  one-way redaction strategy.
- Bind JWT issuance to the verified internal user, not to a frontend address
  or account-list result.

## 9. Multiple-account handling

The safe invariant is:

```text
address(verified publicKey) == server challenge claimed_address
```

That proves which key signed the message after the fact. It does not select a
signer before the call. Since the installed Mini App `sign()` signature has no
address parameter and the official Mini App API does not document the native
selection behavior, NIMNear must not:

- trust `accounts[0]` as the authenticated account;
- accept a frontend-provided address without deriving it from `publicKey`;
- invent an account index, wallet selector, or device binding protocol;
- create a challenge for one account and accept a signature derived from
  another account.

The implementation gate is an official provider behavior or API contract that
lets the user/native host identify the signer. Once that is available, the
frontend can request a challenge for that account and the backend can enforce
the equality above.

## 10. JWT integration plan

Keep the existing HS256 JWT/session infrastructure and protected endpoints.
Add a separate Nimiq authentication use case that calls the existing token
service only after cryptographic verification and atomic challenge
consumption.

The JWT subject remains the internal `users.id`; a NQ address must never become
the primary user identifier. A future token may include a non-authoritative
`auth_method: "nimiq"` claim, but authorization must continue to use the
internal user, roles, permissions, and tenant relationships.

Native-only users need a product-approved profile bootstrap because the current
users table and `UserInfo` DTO assume email. Their display name/username must
not be fabricated from an address or email. Legacy users may link a verified
Nimiq identity after authenticating through the existing login flow.

## 11. Legacy-user migration strategy

1. Keep email/password login and existing JWTs working during migration.
2. Add the identity/challenge tables in a later migration only after the
   cryptographic contract is confirmed.
3. Make legacy credential fields compatible with users that have no email, or
   separate credentials from the core user record. Do not insert placeholder
   emails.
4. Let an already authenticated legacy user link one or more verified Nimiq
   identities through the same challenge flow.
5. For new native users, create an internal user only after signature
   verification and uniqueness checks succeed.
6. Do not merge identities by display name, email guess, profile data, or
   device identifier.
7. Preserve `organizer_id`, event ownership, profiles, RSVP records, purchases,
   and JWT subject relationships by keeping `users.id` stable.

## 12. Organizer identity implications

The verified identity table is the future source for a paid event recipient.
An event must reference a verified organizer identity, or resolve the active
verified identity for its organizer under an explicit product rule.

`NIMNEAR_MERCHANT_ADDRESS` may remain a development/test integration setting,
but it is not the final organizer payout architecture. A profile-editable
address must never become a payment recipient merely because it is stored in a
public profile.

This contract does not change payment code.

## 13. Test plan before a GO verdict

### Contract vectors

- Obtain official Mini App signed-message test vectors containing the exact
  input message, signed bytes/digest, public key, signature, and derived NQ
  address.
- Verify positive and negative vectors in Go.
- Verify altered message, altered prefix/length, altered public key, altered
  signature, malformed hex, zero key, and address mismatch cases.
- Cross-check Go-derived addresses with official `PublicKey.toAddress()`.

### Provider behavior

- Test plain UTF-8 text and the documented `isHex` mode separately.
- Confirm native confirmation/cancellation and both `ErrorResponse` and thrown
  error behavior.
- Approve two accounts and prove which native account signs a challenge.
- Confirm that the returned public key always derives to the signer shown by
  the native UI.
- Confirm the configured TestAlbatross/MainAlbatross network behavior.

### Backend security

- Challenge expiration and clock skew policy.
- Exact message equality and canonicalization rejection.
- Single-use under concurrent verification.
- Rate limiting and generic error responses.
- Unique identity insertion under concurrent first login.
- JWT issuance only after verification.
- Existing email/password login and protected routes remain unchanged.

No E2E success should be simulated before these facts and vectors are
available.

## 14. GO / NO-GO verdict

**NO-GO.** Do not implement NIMNear Nimiq authentication yet.

The following exact information is still required from official Nimiq Mini App
documentation/source or an official maintainer-confirmed test vector:

1. the exact bytes/digest signed by Mini App provider `sign(message)`;
2. the exact semantics of `{ message, isHex }`;
3. the exact error/cancellation contract for the host result path;
4. the native signer-selection rule when multiple accounts are approved, or a
   documented signer/address parameter;
5. confirmation that the public key returned by Mini App `sign()` is the
   ordinary Nimiq Ed25519 public key consumed by `PublicKey.toAddress()`.

The official core source makes the final Go verification path technically
plausible, but it does not close these Mini App-specific gaps. Proceeding
without them would violate the requirement not to invent cryptography or trust
an unbound frontend account claim.
