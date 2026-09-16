export type CollectionState<T> =
  | { kind: "error"; records: [] }
  | { kind: "empty"; records: [] }
  | { kind: "content"; records: T[] };

export function resolveCollectionState<T>(records: T[], error?: string): CollectionState<T> {
  if (error) return { kind: "error", records: [] };
  if (records.length === 0) return { kind: "empty", records: [] };
  return { kind: "content", records };
}
