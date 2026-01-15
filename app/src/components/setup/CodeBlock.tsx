import React from 'react';
import { Clipboard, Check } from 'lucide-react';
import { toast } from 'sonner';

export interface CodeBlockProps {
    value: string;
    accent?: boolean;            // changes text color style (used in browsers / windows)
    className?: string;          // additional container classes
    noWrap?: boolean;            // force single line
    inline?: boolean;            // render inline style (minimal padding, no copy button container sizing change)
    hideCopy?: boolean;          // disable copy button
}

/**
 * Shared CodeBlock component with copy-to-clipboard functionality.
 * Automatically chooses <pre> when multiline content detected.
 */
export const CodeBlock: React.FC<CodeBlockProps> = ({ value, accent = false, className = '', noWrap = false, inline = false, hideCopy = false }) => {
    const [copied, setCopied] = React.useState(false);
    const isMultiline = value.includes('\n');

    const handleCopy = async () => {
        if (hideCopy) return;
        try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
            toast.success('Copied to clipboard');
            setTimeout(() => setCopied(false), 1600);
        } catch {
            toast.error('Copy failed');
        }
    };

    // Reserve space for the copy button so it never overlaps content.
    const copyPadding = hideCopy ? '' : (inline ? 'pr-7' : 'pr-10');

    const baseClasses = inline
        ? `relative inline-flex items-center px-2 py-1 ${copyPadding} rounded border border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-900)] font-mono text-xs ${accent ? 'text-[var(--tailwind-colors-rdns-500)]' : 'text-[var(--tailwind-colors-slate-50)]'} ${noWrap ? 'whitespace-nowrap' : 'break-all'} ${className}`
        : `relative mt-2 rounded border border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-900)] p-2 ${copyPadding || 'pr-10'} font-mono text-xs ${noWrap ? 'whitespace-nowrap overflow-x-auto' : 'break-all'} ${accent ? 'text-[var(--tailwind-colors-rdns-500)]' : 'text-[var(--tailwind-colors-slate-50)]'} ${className}`;

    return (
        <div className={baseClasses}>
            {isMultiline ? (
                <pre className={`leading-relaxed ${noWrap ? 'whitespace-pre' : 'whitespace-pre-wrap'}`}>{value}</pre>
            ) : (
                <code>{value}</code>
            )}
            {!hideCopy && (
                <button
                    type="button"
                    aria-label="Copy value"
                    onClick={handleCopy}
                    className={`absolute ${inline ? 'top-0.5 right-0.5 h-5 w-5' : 'top-1.5 right-1.5 h-6 w-6'} inline-flex items-center justify-center rounded-md border border-[var(--tailwind-colors-slate-700)] bg-[var(--tailwind-colors-slate-900)] text-[var(--tailwind-colors-slate-300)] hover:text-white hover:bg-[var(--tailwind-colors-slate-800)] transition-colors ${copied ? 'border-[var(--tailwind-colors-rdns-600)]' : ''}`}
                >
                    {copied ? <Check className="w-3.5 h-3.5 text-[var(--tailwind-colors-rdns-600)]" /> : <Clipboard className="w-3.5 h-3.5" />}
                </button>
            )}
        </div>
    );
};

export default CodeBlock;
