import assert from "node:assert/strict";
import test from "node:test";

import {
  compactNimiqAddress,
  identiconSeeds,
  normalizeNimiqAddress,
  shortenNimiqAddress,
} from "../../lib/nimiq/address";

test("compacts and shortens Nimiq addresses for the account card", () => {
  const address = "NQ05 U1RF 4MDX SFUP FHYL 9FYC BH7V 5EWN ARYL";
  assert.equal(compactNimiqAddress(address), "NQ05U1RF4MDXSFUPFHYL9FYCBH7V5EWNARYL");
  assert.equal(shortenNimiqAddress(address), "NQ05...YL");
});

test("normalizes valid Nimiq addresses and rejects invalid checksums", () => {
  const official = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604";
  assert.equal(normalizeNimiqAddress("nq46klje5tmf4y1a1255cjhjyg1sh0nut604"), official);
  assert.equal(normalizeNimiqAddress("NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T605"), null);
});

test("the first identicon seed is the wallet address itself", () => {
  const seeds = identiconSeeds("NQ05U1RF4MDXSFUPFHYL9FYCBH7V5EWNARYL", 3);
  assert.equal(seeds[0], "NQ05U1RF4MDXSFUPFHYL9FYCBH7V5EWNARYL");
  assert.equal(seeds.length, 3);
  assert.notEqual(seeds[1], seeds[0]);
});
