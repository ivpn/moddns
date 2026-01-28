import { useTheme } from "next-themes"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { CheckCircle, XCircle, Info } from "lucide-react"

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      style={
        {
          "--normal-bg": "var(--shadcn-ui-app-background)",
          "--normal-text": "var(--tailwind-colors-slate-50)",
          "--normal-border": "var(--tailwind-colors-rdns-600)",
        } as React.CSSProperties
      }
      toastOptions={{
        unstyled: false,
        style: {
          // Responsive width: cap to 100vw minus safe padding; maintain max for desktop
          width: "min(388px, calc(100vw - 24px))",
          padding: "24px 24px 24px 20px",
          gap: "16px",
          borderRadius: "6px",
          boxSizing: 'border-box',
          maxWidth: '100%',
        },
        classNames: {
          toast: "flex items-center gap-4 relative overflow-hidden border-solid bg-[var(--tailwind-colors-rdns-50)] dark:bg-[var(--shadcn-ui-app-background)] text-[var(--shadcn-ui-app-foreground)] dark:text-[var(--tailwind-colors-slate-50)] border border-[var(--tailwind-colors-rdns-600)]",
          title: "text-[var(--shadcn-ui-app-foreground)] dark:text-[var(--tailwind-colors-slate-50)] font-medium",
          description: "text-[var(--tailwind-colors-slate-light-600)] dark:text-[var(--tailwind-colors-slate-200)]",
          success: "!bg-[var(--tailwind-colors-rdns-50)] dark:!bg-[var(--shadcn-ui-app-background)] !border-[var(--tailwind-colors-rdns-600)] [&_[data-description]]:!text-[var(--tailwind-colors-slate-light-700)] dark:[&_[data-description]]:!text-[var(--tailwind-colors-slate-200)]",
          error: "!bg-[var(--tailwind-colors-red-50)] dark:!bg-[var(--shadcn-ui-app-background)] !border-[var(--tailwind-colors-red-600)]",
          info: "!bg-[var(--tailwind-colors-rdns-50)] dark:!bg-[var(--shadcn-ui-app-background)] !border-[var(--tailwind-colors-rdns-600)]",
          warning: "!bg-[var(--tailwind-colors-rdns-50)] dark:!bg-[var(--shadcn-ui-app-background)] !border-[var(--tailwind-colors-rdns-600)]",
        },
      }}
      icons={{
        success: <CheckCircle className="w-6 h-6 text-[var(--tailwind-colors-rdns-600)]" />,
        error: <XCircle className="w-6 h-6 text-[var(--tailwind-colors-red-600)]" />,
        info: <Info className="w-6 h-6 text-[var(--tailwind-colors-rdns-600)]" />,
        warning: <Info className="w-6 h-6 text-[var(--tailwind-colors-rdns-600)]" />,
      }}
      {...props}
    />
  )
}

export { Toaster, Sonner }
