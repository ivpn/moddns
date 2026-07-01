import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Subtle "raise/grow on hover" affordance shared by interactive cards (setup platform cards, query-log rows).
export const INTERACTIVE_CARD =
  "transition-all duration-300 cursor-pointer hover:scale-[1.02] active:scale-100 motion-reduce:transform-none motion-reduce:transition-none";
