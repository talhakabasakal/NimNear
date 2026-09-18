import { compactNimiqAddress, isSameNimiqAddress, normalizeNimiqAddress } from "../nimiq/address";
import {
  lunasToNimString,
  MAX_SAFE_LUNA,
  nimToLunas,
  parseBalanceLunas,
} from "../nimiq/amount";

export type SendAmountError =
  | "empty"
  | "malformed"
  | "too_many_decimals"
  | "zero"
  | "too_large"
  | "exceeds_balance";

export type SendRecipientError = "empty" | "malformed" | "checksum" | "self_send";

export type ValidatedWalletSend = {
  from: string;
  recipient: string;
  amountNim: string;
  amountLunas: bigint;
};

const AMOUNT_MESSAGES: Record<SendAmountError, string> = {
  empty: "Enter an amount.",
  malformed: "Enter a valid NIM amount.",
  too_many_decimals: "NIM amounts can have at most 5 decimal places.",
  zero: "Enter an amount greater than zero.",
  too_large: "This amount is too large to send from this app.",
  exceeds_balance: "Amount exceeds available balance.",
};

const RECIPIENT_MESSAGES: Record<SendRecipientError, string> = {
  empty: "Enter a Nimiq address.",
  malformed: "Enter a valid Nimiq address.",
  checksum: "This Nimiq address has an invalid checksum.",
  self_send: "You cannot send NIM to the same wallet address.",
};

export function amountErrorMessage(error: SendAmountError) {
  return AMOUNT_MESSAGES[error];
}

export function recipientErrorMessage(error: SendRecipientError) {
  return RECIPIENT_MESSAGES[error];
}

export function inspectSendAmount(value: string): SendAmountError | null {
  const normalized = value.trim();
  if (!normalized) return "empty";
  if (!/^[0-9]+(?:\.[0-9]*)?$/.test(normalized)) return "malformed";
  const [, fractionPart = ""] = normalized.split(".");
  if (fractionPart.length > 5) return "too_many_decimals";
  if (normalized.endsWith(".")) return "malformed";
  const lunas = nimToLunas(normalized);
  if (lunas === null) return "malformed";
  if (lunas <= BigInt(0)) return "zero";
  if (lunas > MAX_SAFE_LUNA) return "too_large";
  return null;
}

export function inspectSendRecipient(value: string, fromAddress: string): SendRecipientError | null {
  const trimmed = value.trim();
  if (!trimmed) return "empty";
  const compact = compactNimiqAddress(trimmed);
  if (compact.length !== 36 || !compact.startsWith("NQ")) return "malformed";
  const recipient = normalizeNimiqAddress(trimmed);
  if (!recipient) return "checksum";
  if (isSameNimiqAddress(recipient, fromAddress)) return "self_send";
  return null;
}

export function validateWalletSend(input: {
  recipient: string;
  amount: string;
  fromAddress: string;
  balanceLunas?: string;
}):
  | { ok: true; value: ValidatedWalletSend }
  | { ok: false; field: "recipient" | "amount"; message: string } {
  const recipientError = inspectSendRecipient(input.recipient, input.fromAddress);
  if (recipientError) {
    return { ok: false, field: "recipient", message: recipientErrorMessage(recipientError) };
  }
  const amountError = inspectSendAmount(input.amount);
  if (amountError) {
    return { ok: false, field: "amount", message: amountErrorMessage(amountError) };
  }
  const recipient = normalizeNimiqAddress(input.recipient);
  const from = normalizeNimiqAddress(input.fromAddress) ?? formatFallbackFrom(input.fromAddress);
  const lunas = nimToLunas(input.amount);
  if (!recipient || lunas === null) {
    return { ok: false, field: "recipient", message: recipientErrorMessage("malformed") };
  }
  const available = input.balanceLunas == null ? null : parseBalanceLunas(input.balanceLunas);
  if (available != null && lunas > available) {
    return { ok: false, field: "amount", message: amountErrorMessage("exceeds_balance") };
  }
  return {
    ok: true,
    value: {
      from,
      recipient,
      amountNim: lunasToNimString(lunas),
      amountLunas: lunas,
    },
  };
}

function formatFallbackFrom(fromAddress: string) {
  return fromAddress.trim();
}

export function formatWalletNetwork(network?: string) {
  const compact = network?.trim().toLowerCase().replace(/[-_\s]/g, "") ?? "";
  if (compact === "testalbatross" || compact === "testnet") return "TestAlbatross";
  if (compact === "mainalbatross" || compact === "mainnet") return "MainAlbatross";
  return network?.trim() || "Nimiq";
}

export function requestedWalletAddress(queryAddress?: string | null, sessionAddress?: string | null) {
  const query = queryAddress?.trim();
  if (query) return query;
  return sessionAddress?.trim() || "";
}

export function selectedWalletAddress(input: {
  queryAddress?: string | null;
  sessionAddress?: string | null;
  authorizedAddress?: string | null;
}) {
  const authorized = input.authorizedAddress?.trim();
  if (authorized) return authorized;
  return requestedWalletAddress(input.queryAddress, input.sessionAddress);
}

export function receiveAddressView(address: string) {
  const normalized = normalizeNimiqAddress(address);
  const display = normalized ?? address.trim();
  return {
    display,
    copy: display,
    identiconSeed: compactNimiqAddress(display),
  };
}
