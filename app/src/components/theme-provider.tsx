import { createContext, useContext, useEffect, useState } from "react"

type Theme = "dark" | "light"

type ThemeProviderProps = {
    children: React.ReactNode
    defaultTheme?: Theme
    storageKey?: string
}

type ThemeProviderState = {
    theme: Theme
    setTheme: (theme: Theme) => void
}

const initialState: ThemeProviderState = {
    theme: "dark",
    setTheme: () => null,
}

const ThemeProviderContext = createContext<ThemeProviderState>(initialState)

export function ThemeProvider({
    children,
    defaultTheme = "dark",
    storageKey = "vite-ui-theme",
    ...props
}: ThemeProviderProps) {
    const [theme, setTheme] = useState<Theme>(
        () => {
            const stored = localStorage.getItem(storageKey)
            if (stored === "dark" || stored === "light") return stored
            return defaultTheme
        }
    )

    useEffect(() => {
        const root = window.document.documentElement

        // Disable all CSS transitions so the theme switch is instant
        const style = document.createElement("style")
        style.appendChild(document.createTextNode(
            "*, *::before, *::after { transition: none !important; }"
        ))
        document.head.appendChild(style)

        // Only remove the opposite class to avoid flash when :root:not(.dark) briefly matches
        if (theme === "dark") {
            root.classList.remove("light")
        } else {
            root.classList.remove("dark")
        }
        root.classList.add(theme)

        // Also set the data-shadcn-ui-mode attribute for CSS variable overrides
        document.body.setAttribute(
            'data-shadcn-ui-mode',
            theme === 'dark' ? 'dark-emerald' : 'light-emerald'
        )

        // Force reflow so the browser paints with transitions disabled
        document.body.offsetHeight // eslint-disable-line @typescript-eslint/no-unused-expressions

        // Re-enable transitions on the next frame
        requestAnimationFrame(() => {
            document.head.removeChild(style)
        })
    }, [theme])

    const value = {
        theme,
        setTheme: (theme: Theme) => {
            localStorage.setItem(storageKey, theme)
            setTheme(theme)
        },
    }

    return (
        <ThemeProviderContext.Provider {...props} value={value}>
            {children}
        </ThemeProviderContext.Provider>
    )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useTheme = () => {
    const context = useContext(ThemeProviderContext)

    if (context === undefined)
        throw new Error("useTheme must be used within a ThemeProvider")

    return context
}
