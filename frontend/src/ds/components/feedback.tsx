import { HTMLAttributes, ReactNode } from "react";
import { cn } from "../lib";
import { Check, X, AlertCircle, Info } from "lucide-react";

type CalloutTone = "info" | "positive" | "negative" | "warn";

interface CalloutProps extends HTMLAttributes<HTMLDivElement> {
  tone?: CalloutTone;
  title?: string;
}

const toneConfig: Record<CalloutTone, { bg: string; text: string; icon: ReactNode }> = {
  info: { bg: "bg-accent-subtle", text: "text-accent", icon: <Info className="h-4 w-4" /> },
  positive: { bg: "bg-positive-subtle", text: "text-positive", icon: <Check className="h-4 w-4" /> },
  negative: { bg: "bg-negative-subtle", text: "text-negative", icon: <X className="h-4 w-4" /> },
  warn: { bg: "bg-warn-subtle", text: "text-warn", icon: <AlertCircle className="h-4 w-4" /> },
};

export function Callout({ tone = "info", title, className, children, ...props }: CalloutProps) {
  const cfg = toneConfig[tone];
  return (
    <div className={cn("flex gap-3 rounded-ds p-4", cfg.bg, className)} {...props}>
      <div className={cn("mt-0.5 shrink-0", cfg.text)}>{cfg.icon}</div>
      <div className="flex-1">
        {title && <p className={cn("text-sm font-medium", cfg.text)}>{title}</p>}
        <div className={cn("text-sm text-ink", title && "mt-1")}>{children}</div>
      </div>
    </div>
  );
}

export function Spinner({ className }: { className?: string }) {
  return (
    <svg className={cn("animate-spin h-5 w-5 text-ink-muted", className)} viewBox="0 0 24 24" fill="none">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  );
}

export function Table({ columns, rows }: { columns: { key: string; label: string; className?: string }[]; rows: Record<string, ReactNode>[] }) {
  return (
    <div className="overflow-x-auto rounded-ds border border-edge">
      <table className="w-full text-sm">
        <thead className="bg-canvas">
          <tr>
            {columns.map((col) => (
              <th key={col.key} className={cn("px-4 py-3 text-left font-medium text-ink-muted", col.className)}>
                {col.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-edge">
          {rows.map((row, i) => (
            <tr key={i} className="hover:bg-canvas/50">
              {columns.map((col) => (
                <td key={col.key} className={cn("px-4 py-3 text-ink", col.className)}>
                  {row[col.key]}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
