import assert from "node:assert/strict";
import test from "node:test";

import {
  isNetworkFailure,
  userFacingCaughtError,
  userFacingHttpError,
} from "../../lib/api/http-error";

test("maps important HTTP statuses without exposing internals", () => {
  assert.equal(userFacingHttpError(401, "raw"), "Sign in to continue.");
  assert.equal(userFacingHttpError(403, "raw"), "You do not have permission to do that.");
  assert.equal(userFacingHttpError(404, "raw"), "This item could not be found.");
  assert.equal(userFacingHttpError(409, "raw"), "This action conflicts with the current state. Refresh and try again.");
  assert.equal(userFacingHttpError(429, "raw"), "Too many attempts. Please wait and try again.");
  assert.equal(userFacingHttpError(503, "raw"), "The service is temporarily unavailable. Please try again.");
  assert.equal(userFacingHttpError(500, "Could not save."), "Could not save.");
});

test("network failures stop loading and stay retryable", () => {
  assert.equal(isNetworkFailure(new TypeError("Failed to fetch")), true);
  assert.equal(
    userFacingCaughtError(new TypeError("Failed to fetch"), "fallback"),
    "Network error. Check your connection and try again.",
  );
  assert.equal(
    userFacingCaughtError({ status: 429, message: "rate" }, "fallback"),
    "Too many attempts. Please wait and try again.",
  );
});
