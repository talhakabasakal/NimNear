import assert from "node:assert/strict";
import test from "node:test";

import { receiveAddressView, selectedWalletAddress } from "../../lib/wallet/send";

const primary = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const secondary = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000";

test("receive displays and copies the selected verified address", () => {
  const selected = selectedWalletAddress({
    queryAddress: secondary,
    sessionAddress: primary,
    authorizedAddress: secondary,
  });
  const receive = receiveAddressView(selected);
  assert.equal(selected, secondary);
  assert.equal(receive.copy, secondary);
  assert.equal(receive.display, secondary);
  assert.equal(receive.copy.includes("…"), false);
});

test("receive copy uses the full address rather than a shortened display", () => {
  const receive = receiveAddressView(primary);
  assert.equal(receive.copy, primary);
  assert.ok(receive.copy.startsWith("NQ46"));
  assert.equal(receive.copy.length > 12, true);
});
