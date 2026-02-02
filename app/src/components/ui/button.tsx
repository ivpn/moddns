import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-all cursor-pointer disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-xs hover:bg-primary/90",
        destructive:
          "bg-destructive text-white shadow-xs hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40 dark:bg-destructive/60",
        outline:
          "border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:bg-input/30 dark:border-input dark:hover:bg-input/50",
        secondary:
          "bg-secondary text-secondary-foreground shadow-xs hover:bg-secondary/80",
        cancel:
          "bg-[var(--shadcn-ui-app-background)] border border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-rdns-600)] hover:border-[var(--tailwind-colors-slate-400)] hover:bg-[var(--shadcn-ui-app-background)] focus-visible:ring-0 focus-visible:border-[var(--tailwind-colors-slate-400)] focus:outline-none",
        ghost:
          "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        // Enforce min-h-11 (≈44px) indirectly: keep existing explicit heights for layout but add min-h-11 so customized padding/line-height can't shrink below.
        // On mobile & tablet ensure 44px min tap target; on desktop revert to original visual heights.
        default: "h-9 sm:h-9 lg:h-9 min-h-11 lg:min-h-0 px-4 py-2 has-[>svg]:px-3",
        sm: "h-8 lg:h-8 min-h-11 lg:min-h-0 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5",
        lg: "h-10 lg:h-10 min-h-11 lg:min-h-0 rounded-md px-6 has-[>svg]:px-4",
        icon: "size-9 min-h-11 lg:min-h-0",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
  VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ children, className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"

    return (
      <Comp
        ref={ref}
        data-slot="button"
        data-variant={variant}
        className={cn(buttonVariants({ variant, size, className }))}
        {...props}
      >
        {children}
      </Comp>
    )
  }
)

Button.displayName = "Button"

// eslint-disable-next-line react-refresh/only-export-components
export { Button, buttonVariants }
