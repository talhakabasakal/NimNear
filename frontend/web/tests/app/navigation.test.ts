import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

import { appNavigation, isAppNavActive } from "../../lib/app/navigation";

test("primary navigation is Events, Calendars, then Create", () => {
  assert.deepEqual(
    appNavigation.map((item) => [item.href, item.label]),
    [
      ["/", "Events"],
      ["/calendars", "Calendars"],
      ["/events/create", "Create"],
    ],
  );
});

test("primary navigation does not include Discover or Wallet", () => {
  const labels = appNavigation.map((item) => item.label);
  const hrefs = appNavigation.map((item) => item.href);

  assert.equal(labels.includes("Discover"), false);
  assert.equal(labels.includes("Wallet"), false);
  assert.equal(hrefs.includes("/wallet"), false);
  assert.equal(hrefs.includes("/discover"), false);
});

test("Events is active on the home, events list, and event detail routes", () => {
  assert.equal(isAppNavActive("/", "/"), true);
  assert.equal(isAppNavActive("/events", "/"), true);
  assert.equal(isAppNavActive("/events?view=past", "/"), true);
  assert.equal(isAppNavActive("/events/abc", "/"), true);
  assert.equal(isAppNavActive("/events/create", "/"), false);
  assert.equal(isAppNavActive("/calendars", "/"), false);
  assert.equal(isAppNavActive("/wallet", "/"), false);
});

test("Calendars and Create use prefix matching without overlapping Events", () => {
  assert.equal(isAppNavActive("/calendars", "/calendars"), true);
  assert.equal(isAppNavActive("/calendars/abc", "/calendars"), true);
  assert.equal(isAppNavActive("/events/create", "/events/create"), true);
  assert.equal(isAppNavActive("/events/abc", "/events/create"), false);
  assert.equal(isAppNavActive("/", "/events/create"), false);
});

test("the wallet page remains in the app while Discover redirects to Events", () => {
  const walletPage = fs.readFileSync(
    new URL("../../app/wallet/page.tsx", import.meta.url),
    "utf8",
  );
  const nextConfig = fs.readFileSync(
    new URL("../../next.config.ts", import.meta.url),
    "utf8",
  );

  assert.match(walletPage, /WalletScreen/);
  assert.match(nextConfig, /source: "\/discover"/);
  assert.match(nextConfig, /destination: "\/"/);
});
