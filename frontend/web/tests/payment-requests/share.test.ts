import assert from "node:assert/strict";
import test from "node:test";

import {
  appOrigin,
  canUseWebShare,
  copyText,
  paymentRequestPath,
  paymentRequestShareText,
  paymentRequestShareUrl,
  sharePaymentRequest,
  shareUrlContainsSecrets,
} from "../../lib/payment-requests/share";

const publicId = "11111111-1111-4111-8111-111111111111";

test("canonical share URL uses the application origin and /pay/{public_id}", () => {
  assert.equal(paymentRequestPath(publicId), `/pay/${publicId}`);
  assert.equal(
    paymentRequestShareUrl(publicId, "https://app.nimnear.example"),
    `https://app.nimnear.example/pay/${publicId}`,
  );
  assert.equal(appOrigin({ origin: "https://pay.example" }), "https://pay.example");
  assert.equal(paymentRequestShareUrl(publicId, "https://app.nimnear.example").includes("localhost"), false);
  assert.equal(shareUrlContainsSecrets(paymentRequestShareUrl(publicId, "https://app.nimnear.example")), false);
});

test("production share URLs reject HTTP and Vercel preview origins", () => {
  assert.equal(paymentRequestShareUrl(publicId, "http://localhost:3000", "production"), "");
  assert.equal(paymentRequestShareUrl(publicId, "https://nimnear-git-main.vercel.app", "production"), "");
  assert.equal(
    paymentRequestShareUrl(publicId, "https://app.example.com", "production"),
    `https://app.example.com/pay/${publicId}`,
  );
  assert.equal(paymentRequestShareUrl(publicId, "http://localhost:3000", "development"), `http://localhost:3000/pay/${publicId}`);
});

test("copy link writes only the canonical HTTPS URL", async () => {
  const written: string[] = [];
  await copyText(
    paymentRequestShareUrl(publicId, "https://nimnear.example"),
    { writeText: async (value) => { written.push(value); } },
  );
  assert.deepEqual(written, [`https://nimnear.example/pay/${publicId}`]);
});

test("Web Share uses concise public content and copy remains available without it", async () => {
  assert.equal(canUseWebShare(undefined), false);
  assert.equal(paymentRequestShareText({ amountNim: "25", note: "Dinner" }).includes("Dinner"), true);
  assert.equal(paymentRequestShareText({ amountNim: "25", note: "Dinner" }).toLowerCase().includes("jwt"), false);
  let shared: ShareData | undefined;
  const used = await sharePaymentRequest(
    {
      title: "NIMNear payment request",
      text: paymentRequestShareText({ amountNim: "25", note: "Dinner" }),
      url: paymentRequestShareUrl(publicId, "https://nimnear.example"),
    },
    async (data) => {
      shared = data;
    },
  );
  assert.equal(used, true);
  assert.equal(shared?.url, `https://nimnear.example/pay/${publicId}`);
  assert.equal(String(shared?.url).includes("token="), false);
});
