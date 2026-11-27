import { describe, it, expect } from "vitest";
import { buildOpenWrtCommands } from "@/components/setup/routerCommands";

const DOH = "https://dns.moddns.net/dns-query/abc123def4";

describe("buildOpenWrtCommands", () => {
    it("points https-dns-proxy at the profile's DoH endpoint", () => {
        const cmds = buildOpenWrtCommands({ dohEndpoint: DOH });
        expect(cmds).toContain(`uci set https-dns-proxy.dns.resolver_url="${DOH}"`);
        expect(cmds).toContain('uci set https-dns-proxy.dns.listen_port="5053"');
        expect(cmds.trimEnd().endsWith("service https-dns-proxy restart")).toBe(true);
    });

    it("leaves bootstrap_dns at the package default", () => {
        // The bootstrap lookup is plain DNS; modDNS answers encrypted DNS only.
        const cmds = buildOpenWrtCommands({ dohEndpoint: DOH });
        expect(cmds).not.toMatch(/bootstrap_dns/);
    });

    it("covers both package managers", () => {
        const cmds = buildOpenWrtCommands({ dohEndpoint: DOH });
        expect(cmds).toContain("opkg install https-dns-proxy");
        expect(cmds).toContain("apk add https-dns-proxy");
    });
});
