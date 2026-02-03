type LogoVariants = {
    base?: string;
    white?: string;
    dark?: string;
};

// TODO: Consider lazy-loading service logos when code splitting is complete
function buildServiceLogoMap(): Record<string, LogoVariants> {
    const svgModules = import.meta.glob("./*.svg", { eager: true, import: "default" }) as Record<
        string,
        string
    >;
    const pngModules = import.meta.glob("./*.png", { eager: true, import: "default" }) as Record<
        string,
        string
    >;

    const allModules: Record<string, string> = { ...svgModules, ...pngModules };
    const map: Record<string, LogoVariants> = {};

    for (const [path, src] of Object.entries(allModules)) {
        const file = path.split("/").pop();
        if (!file) continue;

        const baseName = file.replace(/\.(svg|png)$/i, "");

        let variant: keyof LogoVariants = "base";
        let servicePart = baseName;

        if (baseName.endsWith("_white")) {
            variant = "white";
            servicePart = baseName.slice(0, -"_white".length);
        } else if (baseName.endsWith("_dark")) {
            variant = "dark";
            servicePart = baseName.slice(0, -"_dark".length);
        }

        const key = normalizeServiceKey(servicePart);
        if (!key) continue;

        map[key] ??= {};
        map[key][variant] = src;
    }

    return map;
}

const SERVICE_LOGOS: Record<string, LogoVariants> = buildServiceLogoMap();

function normalizeServiceKey(value: string): string {
    return value.trim().toLowerCase().replace(/[^a-z0-9]/g, "");
}

export function getServiceLogoSrc(params: {
    serviceId?: string | null;
    serviceName?: string | null;
}): string | undefined {
    const raw = params.serviceId || params.serviceName;
    if (!raw) return undefined;

    const key = normalizeServiceKey(raw);
    const variants = SERVICE_LOGOS[key];
    if (!variants) return undefined;

    const isDarkModeEnabled =
        typeof document !== "undefined" &&
        document.documentElement.classList.contains("dark");

    // Dark mode: prefer white logo; Light mode: prefer dark logo.
    if (isDarkModeEnabled) return variants.white ?? variants.base ?? variants.dark;
    return variants.dark ?? variants.base ?? variants.white;
}
