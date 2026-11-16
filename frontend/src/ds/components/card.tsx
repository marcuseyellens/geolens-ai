import { HTMLAttributes } from "react";
import { cn } from "../lib";

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  raised?: boolean;
}

export function Card({ className, raised, ...props }: CardProps) {
  return (
    <div
      className={cn(
        "relative rounded-ds-lg bg-surface border border-edge p-5 overflow-hidden",
        "before:absolute before:inset-0 before:bg-[image:var(--ds-gradient-surface)] before:pointer-events-none",
        raised && "shadow-lg shadow-accent/5 border-edge-strong",
        className,
      )}
      {...props}
    />
  );
}

export function Badge({
  children,
  tone = "neutral",
  className,
}: {
  children: React.ReactNode;
  tone?: "neutral" | "positive" | "negative" | "warn" | "accent";
  className?: string;
}) {
  const tones: Record<string, string> = {
    neutral: "bg-canvas text-ink-muted border-edge",
    positive: "bg-positive-subtle text-positive border-positive/20",
    negative: "bg-negative-subtle text-negative border-negative/20",
    warn: "bg-warn-subtle text-warn border-warn/20",
    accent: "bg-accent-subtle text-accent border-accent/20",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center px-2 py-0.5 text-xs font-medium rounded-ds-sm border",
        tones[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}

export function StatCard({
  label,
  value,
  hint,
  icon,
}: {
  label: string;
  value: string | number;
  hint?: string;
  icon?: React.ReactNode;
}) {
  return (
    <Card raised>
      <div className="relative flex items-start justify-between">
        <div className="min-w-0">
          <p className="text-sm text-ink-muted">{label}</p>
          <p className="mt-2 text-2xl font-bold text-ink tracking-tight">{value}</p>
          {hint && <p className="mt-1 text-xs text-ink-subtle truncate">{hint}</p>}
        </div>
        {icon && (
          <div className="flex items-center justify-center h-10 w-10 rounded-ds bg-[image:var(--ds-gradient-accent)] text-white shadow-md shadow-accent/20 shrink-0">
            {icon}
          </div>
        )}
      </div>
    </Card>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <p className="text-lg font-medium text-ink">{title}</p>
      {description && <p className="mt-1 text-sm text-ink-muted">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
