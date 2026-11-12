import { HTMLAttributes } from "react";
import { cn } from "../lib";

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  raised?: boolean;
}

export function Card({ className, raised, ...props }: CardProps) {
  return (
    <div
      className={cn(
        "rounded-ds-lg bg-surface border border-edge p-5",
        raised && "shadow-sm",
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
    positive: "bg-positive-subtle text-positive border-transparent",
    negative: "bg-negative-subtle text-negative border-transparent",
    warn: "bg-warn-subtle text-warn border-transparent",
    accent: "bg-accent-subtle text-accent border-transparent",
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
    <Card>
      <div className="flex items-center justify-between">
        <p className="text-sm text-ink-muted">{label}</p>
        {icon && <div className="text-ink-subtle">{icon}</div>}
      </div>
      <p className="mt-2 text-2xl font-semibold text-ink">{value}</p>
      {hint && <p className="mt-1 text-xs text-ink-subtle">{hint}</p>}
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
