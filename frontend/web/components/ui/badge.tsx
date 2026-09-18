import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex h-6 items-center gap-1.5 rounded-full px-2.5 text-[11px] font-medium",
  {
    variants: {
      variant: {
        default: "bg-primary/15 text-accent",
        success: "bg-emerald-400/15 text-emerald-300",
        warning: "bg-amber-400/15 text-amber-300",
        destructive: "bg-red-400/15 text-red-300",
        outline: "border border-border-faint text-muted",
      },
    },
    defaultVariants: { variant: "default" },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { Badge, badgeVariants };
