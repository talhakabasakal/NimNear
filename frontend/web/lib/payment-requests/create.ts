import { lunasToNimString, nimToLunas } from "../nimiq/amount";
import { amountErrorMessage, inspectSendAmount } from "../wallet/send";

export const PAYMENT_REQUEST_MAX_NOTE_LENGTH = 140;

export type CreatePaymentRequestDraft = {
  amountNim: string;
  note: string | null;
  address: string;
};

export type CreatePaymentRequestField = "amount" | "note" | "identity";

export function inspectPaymentRequestNote(value: string): "too_long" | "control" | null {
  const trimmed = value.trim();
  if ([...trimmed].length > PAYMENT_REQUEST_MAX_NOTE_LENGTH) return "too_long";
  for (const char of trimmed) {
    const code = char.codePointAt(0) ?? 0;
    if (code <= 0x1f || (code >= 0x7f && code <= 0x9f)) return "control";
  }
  return null;
}

export function noteErrorMessage(error: "too_long" | "control") {
  if (error === "too_long") {
    return `Note must be at most ${PAYMENT_REQUEST_MAX_NOTE_LENGTH} characters.`;
  }
  return "Note must be plain text.";
}

export function validateCreatePaymentRequest(input: {
  amount: string;
  note?: string;
  selectedAddress?: string | null;
}):
  | { ok: true; value: CreatePaymentRequestDraft }
  | { ok: false; field: CreatePaymentRequestField; message: string } {
  const selectedAddress = input.selectedAddress?.trim() ?? "";
  if (!selectedAddress) {
    return {
      ok: false,
      field: "identity",
      message: "A verified Nimiq identity is required to create a request.",
    };
  }

  const amountError = inspectSendAmount(input.amount);
  if (amountError) {
    return { ok: false, field: "amount", message: amountErrorMessage(amountError) };
  }
  const lunas = nimToLunas(input.amount);
  if (lunas == null || lunas <= BigInt(0)) {
    return { ok: false, field: "amount", message: amountErrorMessage("malformed") };
  }

  const noteError = inspectPaymentRequestNote(input.note ?? "");
  if (noteError) {
    return { ok: false, field: "note", message: noteErrorMessage(noteError) };
  }

  const note = (input.note ?? "").trim();
  return {
    ok: true,
    value: {
      amountNim: lunasToNimString(lunas),
      note: note.length > 0 ? note : null,
      address: selectedAddress,
    },
  };
}
