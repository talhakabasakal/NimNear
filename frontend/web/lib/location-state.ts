export type LocationState =
  | "idle"
  | "locating"
  | "loading"
  | "denied"
  | "unavailable"
  | "timeout"
  | "error"
  | "empty"
  | "success";

export function locationStateForError(code: number): Extract<LocationState, "denied" | "unavailable" | "timeout"> {
  if (code === 1) return "denied";
  if (code === 3) return "timeout";
  return "unavailable";
}


export type NearbyResultState = "error" | "empty" | "success";

export function nearbyResultState(records: unknown[], error?: boolean): NearbyResultState {
  if (error) return "error";
  return records.length === 0 ? "empty" : "success";
}
