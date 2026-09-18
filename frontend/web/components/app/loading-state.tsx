export function LoadingState({ count = 3 }: { count?: number }) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" aria-label="Loading">
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="overflow-hidden rounded-xl border border-border bg-surface">
          <div className="h-36 animate-pulse bg-surface-hover" />
          <div className="space-y-3 p-4">
            <div className="h-3 w-1/3 animate-pulse rounded bg-surface-hover" />
            <div className="h-5 w-4/5 animate-pulse rounded bg-surface-hover" />
            <div className="h-3 w-2/3 animate-pulse rounded bg-surface-hover" />
          </div>
        </div>
      ))}
    </div>
  );
}
