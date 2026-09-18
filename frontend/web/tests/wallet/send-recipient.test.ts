import assert from "node:assert/strict";
import test from "node:test";

import { normalizeNimiqAddress } from "../../lib/nimiq/address";
import {
  inspectSendRecipient,
  receiveAddressView,
  requestedWalletAddress,
  selectedWalletAddress,
  validateWalletSend,
} from "../../lib/wallet/send";

const from = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const recipient = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000";

test("recipient validation accepts spaced mixed-case valid addresses", () => {
  assert.equal(inspectSendRecipient("nq07 0000 0000 0000 0000 0000 0000 0000 0000", from), null);
  const result = validateWalletSend({
    recipient: "nq0700000000000000000000000000000000",
    amount: "1",
    fromAddress: from,
  });
  assert.equal(result.ok, true);
  if (result.ok) assert.equal(result.value.recipient, recipient);
});

test("recipient validation rejects empty, malformed, checksum, and self-send", () => {
  assert.equal(inspectSendRecipient("", from), "empty");
  assert.equal(inspectSendRecipient("NQ...", from), "malformed");
  assert.equal(inspectSendRecipient("NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T605", from), "checksum");
  assert.equal(inspectSendRecipient(from, from), "self_send");
  assert.equal(inspectSendRecipient("nq46klje5tmf4y1a1255cjhjyg1sh0nut604", from), "self_send");
  assert.equal(normalizeNimiqAddress(from), from);
});

test("receive and send use the selected verified identity, not an injected From field", () => {
  const query = recipient;
  const session = from;
  const authorized = recipient;
  assert.equal(requestedWalletAddress(query, session), query);
  assert.equal(selectedWalletAddress({ queryAddress: query, sessionAddress: session, authorizedAddress: authorized }), authorized);
  const receive = receiveAddressView(authorized);
  assert.equal(receive.copy, recipient);
  assert.equal(receive.display, recipient);
  assert.equal(receive.identiconSeed, "NQ0700000000000000000000000000000000");
});
