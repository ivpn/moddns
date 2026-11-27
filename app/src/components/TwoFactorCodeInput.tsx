import { type ReactNode, useState } from 'react';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { isTwoFactorCodeFormatValid, type TwoFactorCodeMode } from '@/lib/twoFactorCode';

interface TwoFactorCodeInputProps {
    id: string;
    value: string;
    /** Called with the entered text and whether it has the format of the chosen kind of code. */
    onChange: (value: string, valid: boolean) => void;
    /** Called on Enter when the entered code has a valid format. */
    onSubmit?: () => void;
    disabled?: boolean;
    autoFocus?: boolean;
    /** False where only an authenticator code is accepted (confirming a new 2FA set-up). */
    allowBackupCode?: boolean;
    /** 'labelled' shows a label and a format hint (dialogs); 'compact' only a placeholder (sign-in pages). */
    variant?: 'labelled' | 'compact';
    inputClassName?: string;
    /** Icon drawn inside the field on the left; inputClassName must leave room for it. */
    leadingIcon?: ReactNode;
    'data-testid'?: string;
}

const COPY: Record<TwoFactorCodeMode, { label: string; placeholder: string; hint: string; invalid: string; switchTo: string }> = {
    authenticator: {
        label: '2FA code',
        placeholder: '2FA code',
        hint: '6-digit code from your authenticator app',
        invalid: 'Enter the 6-digit code from your authenticator app.',
        switchTo: 'Use your authenticator app instead',
    },
    backup: {
        label: 'Backup code',
        placeholder: 'Backup code',
        hint: 'One of your saved backup codes. Each code works once.',
        invalid: "This doesn't look like a backup code. Check it and try again.",
        switchTo: 'Lost your device? Use a backup code',
    },
};

const fieldClass = 'h-10 border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-50)]';

/** The one field every screen uses to ask for a 2FA code. It takes an authenticator code by default
 *  (numeric keypad, 6 digits) and switches to a backup code (text keyboard, backup-code format). */
export default function TwoFactorCodeInput({
    id,
    value,
    onChange,
    onSubmit,
    disabled = false,
    autoFocus = false,
    allowBackupCode = true,
    variant = 'labelled',
    inputClassName,
    leadingIcon,
    'data-testid': testId,
}: TwoFactorCodeInputProps) {
    const [mode, setMode] = useState<TwoFactorCodeMode>('authenticator');
    const [touched, setTouched] = useState(false);
    const copy = COPY[mode];
    const valid = isTwoFactorCodeFormatValid(value, mode);
    const showInvalid = touched && value.trim() !== '' && !valid;
    const hintId = `${id}-hint`;
    const otherMode: TwoFactorCodeMode = mode === 'authenticator' ? 'backup' : 'authenticator';

    const switchMode = () => {
        setMode(otherMode);
        setTouched(false);
        onChange('', false);
        // Refocus so the keyboard for the new mode opens at once on phones.
        requestAnimationFrame(() => document.getElementById(id)?.focus());
    };

    const input = (
        <Input
            id={id}
            data-testid={testId}
            data-mode={mode}
            type="text"
            inputMode={mode === 'authenticator' ? 'numeric' : 'text'}
            autoComplete="one-time-code"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            maxLength={64}
            placeholder={variant === 'compact' ? copy.placeholder : undefined}
            value={value}
            onChange={e => {
                const next = e.target.value;
                // A backup code pasted into the authenticator field switches the field to it.
                if (mode === 'authenticator' && allowBackupCode && /[A-Za-z]/.test(next)
                    && isTwoFactorCodeFormatValid(next, 'backup')) {
                    setMode('backup');
                    onChange(next, true);
                    return;
                }
                onChange(next, isTwoFactorCodeFormatValid(next, mode));
            }}
            onBlur={() => setTouched(true)}
            onKeyDown={e => {
                if (e.key !== 'Enter') return;
                setTouched(true);
                if (valid && onSubmit) {
                    e.preventDefault();
                    onSubmit();
                }
            }}
            disabled={disabled}
            autoFocus={autoFocus}
            aria-invalid={showInvalid || undefined}
            aria-describedby={hintId}
            aria-label={variant === 'compact' ? copy.label : undefined}
            className={cn(variant === 'labelled' && fieldClass, inputClassName)}
        />
    );

    return (
        <div className="flex flex-col gap-2 w-full">
            {variant === 'labelled' && (
                <label htmlFor={id} className="text-sm font-medium text-[var(--tailwind-colors-slate-50)]">
                    {copy.label}
                </label>
            )}
            {leadingIcon ? (
                <div className="relative w-full">
                    {leadingIcon}
                    {input}
                </div>
            ) : input}
            <p
                id={hintId}
                data-testid={testId ? `${testId}-hint` : undefined}
                className={cn(
                    'text-xs leading-4',
                    showInvalid ? 'text-[var(--tailwind-colors-red-400)]' : 'text-[var(--tailwind-colors-slate-400)]',
                    variant === 'compact' && !showInvalid && 'sr-only',
                )}
            >
                {showInvalid ? copy.invalid : copy.hint}
            </p>
            {allowBackupCode && (
                <button
                    type="button"
                    onClick={switchMode}
                    disabled={disabled}
                    data-testid={testId ? `${testId}-mode-toggle` : undefined}
                    className="self-start -my-2 lg:my-0 text-xs font-medium text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-700)] hover:underline min-h-11 lg:min-h-0 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    {COPY[otherMode].switchTo}
                </button>
            )}
        </div>
    );
}
