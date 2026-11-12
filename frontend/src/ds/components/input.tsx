import { InputHTMLAttributes, TextareaHTMLAttributes, forwardRef, useId } from "react";
import { cn } from "../lib";

interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export const TextField = forwardRef<HTMLInputElement, TextFieldProps>(
  ({ className, label, error, id, ...props }, ref) => {
    const generatedId = useId();
    const fieldId = id ?? generatedId;
    const errorId = `${fieldId}-error`;

    return (
      <div className="space-y-1.5">
        {label && (
          <label htmlFor={fieldId} className="block text-sm font-medium text-ink">
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={fieldId}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          className={cn(
            "w-full px-3 py-2 text-sm rounded-ds bg-surface border border-edge text-ink",
            "placeholder:text-ink-subtle focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent",
            "disabled:opacity-50 disabled:cursor-not-allowed",
            error && "border-negative",
            className,
          )}
          {...props}
        />
        {error && (
          <p id={errorId} className="text-xs text-negative">
            {error}
          </p>
        )}
      </div>
    );
  },
);
TextField.displayName = "TextField";

interface TextAreaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
}

export const TextArea = forwardRef<HTMLTextAreaElement, TextAreaProps>(
  ({ className, label, error, id, ...props }, ref) => {
    const generatedId = useId();
    const fieldId = id ?? generatedId;
    const errorId = `${fieldId}-error`;

    return (
      <div className="space-y-1.5">
        {label && (
          <label htmlFor={fieldId} className="block text-sm font-medium text-ink">
            {label}
          </label>
        )}
        <textarea
          ref={ref}
          id={fieldId}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          className={cn(
            "w-full px-3 py-2 text-sm rounded-ds bg-surface border border-edge text-ink",
            "placeholder:text-ink-subtle focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent",
            "min-h-[80px] resize-y disabled:opacity-50 disabled:cursor-not-allowed",
            error && "border-negative",
            className,
          )}
          {...props}
        />
        {error && (
          <p id={errorId} className="text-xs text-negative">
            {error}
          </p>
        )}
      </div>
    );
  },
);
TextArea.displayName = "TextArea";
