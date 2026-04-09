import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';

/**
 * Reusable status badge component aligned with ConnectionStatusHeader styling.
 * Usage: <StatusBadge intent="success" text="Verified" />
 */
export interface StatusBadgeProps {
    intent: 'success' | 'error' | 'warning' | 'info' | 'neutral';
    text: string;
    size?: 'xs' | 'sm';
    className?: string;
    'data-testid'?: string;
}

const intentStyles: Record<StatusBadgeProps['intent'], string> = {
    success: 'bg-[var(--tailwind-colors-rdns-600)] border border-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-700)]',
    error: '!bg-[var(--tailwind-colors-red-600)] border border-[var(--tailwind-colors-red-600)] text-white',
    warning: 'bg-[var(--tailwind-colors-orange-500)]/30 border border-[var(--tailwind-colors-orange-500)] text-[var(--tailwind-colors-orange-300)]',
    info: 'bg-[var(--tailwind-colors-rdns-600)]/30 border border-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-50)]',
    neutral: 'bg-[var(--tailwind-colors-slate-600)]/30 border border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)]',
};

const sizeStyles: Record<NonNullable<StatusBadgeProps['size']>, string> = {
    xs: 'px-2 py-0.5 text-[10px] font-semibold',
    sm: 'px-2.5 py-0.5 text-xs font-semibold',
};

export function StatusBadge({ intent, text, size = 'sm', className, 'data-testid': testId }: StatusBadgeProps) {
    return (
        <Badge
            data-testid={testId}
            className={cn('rounded', intentStyles[intent], sizeStyles[size], className)}
        >
            {text}
        </Badge>
    );
}

export default StatusBadge;
