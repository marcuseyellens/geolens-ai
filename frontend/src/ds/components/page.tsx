import { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";
import { Link } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { cn } from "../lib";

const BACK_STYLE =
  "inline-flex items-center gap-1.5 text-sm text-ink-muted hover:text-ink";

interface PageProps extends HTMLAttributes<HTMLDivElement> {
  measure?: "narrow" | "default" | "wide";
}

const measures: Record<string, string> = {
  narrow: "max-w-2xl",
  default: "max-w-6xl",
  wide: "max-w-[1600px]",
};

export function Page({ className, measure = "default", children, ...props }: PageProps) {
  return (
    <div className={cn("mx-auto px-6 py-8", measures[measure], className)} {...props}>
      {children}
    </div>
  );
}

export function PageHeader({
  title,
  description,
  actions,
  back,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  /** 返回导航独占一行，位于标题和 actions 之上。 */
  back?: ReactNode;
}) {
  return (
    <header className="mb-6">
      {back && <div className="mb-3">{back}</div>}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-xl font-semibold text-ink">{title}</h1>
          {description && <p className="mt-1 text-sm text-ink-muted">{description}</p>}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
    </header>
  );
}

/** 返回站内某个路由，14px 左箭头 + 弱化文字，与标题不同行。 */
export function PageBackLink({
  to,
  children,
  className,
}: {
  to: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Link to={to} className={cn(BACK_STYLE, className)}>
      <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
      {children}
    </Link>
  );
}

/** 需要执行回调再离开时使用。 */
export function PageBackButton({
  children,
  className,
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button type={type} className={cn(BACK_STYLE, className)} {...props}>
      <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
      {children}
    </button>
  );
}
