import assert from "node:assert/strict";
import test from "node:test";

import { validateCreatePaymentRequest } from "../../lib/payment-requests/create";
import { selectedWalletAddress } from "../../lib/wallet/send";

const primary = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
const secondary = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000";

test("payment requests use the selected verified identity rather than arbitrary recipient input", () => {
  const selected = selectedWalletAddress({
    queryAddress: secondary,
    sessionAddress: primary,
    authorizedAddress: secondary,
  });
  const result = validateCreatePaymentRequest({
    amount: "10",
    selectedAddress: selected,
  });
  assert.equal(result.ok, true);
  if (result.ok) {
    assert.equal(result.value.address, secondary);
    assert.notEqual(result.value.address, primary);
  }
  const missing = validateCreatePaymentRequest({ amount: "10", selectedAddress: "" });
  assert.equal(missing.ok, false);
  if (!missing.ok) assert.equal(missing.field, "identity");
});
