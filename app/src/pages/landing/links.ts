// Centralised external URLs surfaced on the marketing landing page.
//
// IVPN paths derive from VITE_IVPN_HOME_URL (set in app/env/.env.*) so prod /
// staging / local can repoint coherently with one env-file edit. GitHub URLs
// are env-invariant — the canonical repos don't change between deployments.

const ivpnHome = (import.meta.env.VITE_IVPN_HOME_URL || 'https://www.ivpn.net')
    .replace(/\/+$/, ''); // tolerate trailing slash in env value

export const LINKS = Object.freeze({
    ivpnHome,
    pricing: `${ivpnHome}/pricing/`,
    ivpnTeam: `${ivpnHome}/en/team/`,
    unlinkedAccess: `${ivpnHome}/en/unlinked-access/`,
    auditReport: `${ivpnHome}/resources/IVP-08-report.pdf`,
    moddnsRepo: 'https://github.com/ivpn/moddns',
    unlinkedRepo: 'https://github.com/ivpn/unlinked-access',
});

export type LandingLinks = typeof LINKS;
