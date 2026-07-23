import { describe, it, expect } from "vitest";
import { buildDnscryptProxyToml } from "@/components/setup/dnscryptProxy";

const STAMP = "sdns://AgcAAAAAAAAAAA0xLjEuMS4xAA5kbnMubW9kZG5zLm5ldBYvZG5zLXF1ZXJ5L2FiYzEyM2RlZjQ";

describe("buildDnscryptProxyToml", () => {
    it("builds a static server entry keyed on a modDNS-<profileId> label", () => {
        const toml = buildDnscryptProxyToml("abc123def4", STAMP);
        expect(toml).toContain("server_names = ['modDNS-abc123def4']");
        expect(toml).toContain("[static]");
        expect(toml).toContain("[static.'modDNS-abc123def4']");
        expect(toml).toContain(`stamp = '${STAMP}'`);
    });

    it("uses the same server name in both the list and the static section", () => {
        const toml = buildDnscryptProxyToml("xyz789", STAMP);
        const matches = toml.match(/modDNS-xyz789/g) ?? [];
        expect(matches).toHaveLength(2);
    });
});
