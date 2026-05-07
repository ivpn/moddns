import { Link } from 'react-router-dom';
import SystemClock from './SystemClock';
import TopologyDiagram from './TopologyDiagram';
import { LINKS } from './links';
import dashboardScreenshot from '@/assets/landing/dashboard-screenshot.png';
import dashboardScreenshotMobile from '@/assets/landing/dashboard-screenshot-mobile.png';
import './landing.css';

type LandingProps = {
    /**
     * When true, the page swaps the [01 LOGIN] CTA for [01 DASHBOARD] (linking
     * to /home). Authenticated users still see the marketing page at `/`;
     * this prop simply gives them a direct path back into the app.
     *
     * Defaults to `false` so the component stays trivially renderable in
     * tests, Storybook, etc. — the auth-aware caller (RootIndexRedirect) is
     * responsible for passing the real value.
     */
    isAuthenticated?: boolean;
};

export default function Landing({ isAuthenticated = false }: LandingProps) {
    return (
        <div className="moddns-landing">
            <div className="container">

                {/* NAV */}
                <header className="sys-nav">
                    <div>
                        <span>
                            {isAuthenticated ? (
                                <Link to="/home" style={{ color: 'inherit', textDecoration: 'none' }}>
                                    [01 DASHBOARD]
                                </Link>
                            ) : (
                                <Link to="/login" style={{ color: 'inherit', textDecoration: 'none' }}>
                                    [01 LOGIN]
                                </Link>
                            )}
                        </span>
                        <span>
                            <a
                                href={LINKS.pricing}
                                target="_blank"
                                rel="noopener noreferrer"
                                style={{ color: 'inherit', textDecoration: 'none' }}
                            >
                                [02 START]
                            </a>
                        </span>
                        <span>
                            <Link to="/privacy" style={{ color: 'inherit', textDecoration: 'none' }}>
                                [03 PRIVACY]
                            </Link>
                        </span>
                        <span>
                            <Link to="/faq" style={{ color: 'inherit', textDecoration: 'none' }}>
                                [04 FAQ]
                            </Link>
                        </span>
                    </div>
                    <div>
                        <span>STATUS: ONLINE</span>
                        <span>SYS.TIME: <SystemClock /></span>
                    </div>
                </header>

                {/* HERO */}
                <section className="hero window">
                    <div className="section-title">MODDNS_LOADED</div>
                    <div className="hero-layout">
                        <div>
                            <h1 className="glitch-block" data-text="RESIST_DNS_SURVEILLANCE">
                                <span className="hero-h1-part">RESIST_DNS_</span><span className="hero-h1-part">SURVEILLANCE<span className="blink">_</span></span>
                            </h1>
                            <p>
                                Block ads and trackers at the DNS level using modDNS, an open-source
                                service with configurable blocklists and custom rules.
                            </p>
                            <div className="btn-group">
                                <a
                                    href={LINKS.pricing}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="btn"
                                >
                                    [ START ]
                                </a>
                                <a
                                    href={LINKS.moddnsRepo}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="btn"
                                >
                                    [ VIEW SOURCE ]
                                </a>
                            </div>
                        </div>
                        <p className="hero-explain" style={{ color: '#fff' }}>
                            Your DNS queries reveal which domains you visit. ISPs can monitor them, and
                            websites use them to share data with third-party ad and tracking networks.
                            While using your VPN provider's DNS resolver and tracker-blocker tool can
                            address this issue, modDNS offers more visibility and better control over
                            what is blocked.
                        </p>
                    </div>
                    {/* TODO(landing): pull v.1.0.6 from package.json or a build-time constant */}
                    <span className="meta-data meta-bl">v.1.0.6</span>
                </section>

                {/* PRODUCT SCREENSHOT */}
                <section className="window">
                    <div className="window-title">[ MODDNS_DASHBOARD ]</div>
                    <div className="screenshot-container">
                        <picture>
                            {/* Mobile-tuned screenshot at ≤768 px (smaller, narrower aspect).
                                Desktop falls back to the wide screenshot below. */}
                            <source
                                media="(max-width: 768px)"
                                srcSet={dashboardScreenshotMobile}
                            />
                            <img
                                src={dashboardScreenshot}
                                alt="modDNS Dashboard — Custom Rules Interface"
                                loading="lazy"
                                decoding="async"
                            />
                        </picture>
                    </div>
                </section>

                {/* WHAT YOU CAN DO */}
                <section>
                    <div className="section-title">DNS_FILTER_MODULES</div>
                    <h2 style={{ fontSize: '2.5rem', marginBottom: '1.5rem' }}>
                        Granular DNS Filtering
                    </h2>
                    <div className="features-grid">
                        <div className="feature-item">
                            <h3>01. COMBINE BLOCKLISTS</h3>
                            <p style={{ color: '#fff' }}>
                                Start with Basic protection or Comprehensive/Restrictive presets. Enable
                                individual lists from Hagezi, OISD, and others. Block specific services
                                (e.g. Facebook, Google, Amazon) or categories (e.g. adult content,
                                gambling).
                            </p>
                        </div>
                        <div className="feature-item">
                            <h3>02. DEFINE CUSTOM RULES</h3>
                            <p style={{ color: '#fff' }}>
                                Override blocklists with allowlist entries for domains you don't want
                                blocked. Add denylist entries for domains not covered by existing lists.
                                Use wildcard patterns for wider coverage.
                            </p>
                        </div>
                        <div className="feature-item">
                            <h3>03. CREATE DNS PROFILES</h3>
                            <p style={{ color: '#fff' }}>
                                Configure different filtering rules for work and personal devices. Each
                                profile gets a unique identifier for DNS setup. Supports DNS-over-HTTPS,
                                DNS-over-TLS, and DNS-over-QUIC.
                            </p>
                        </div>
                        <div className="feature-item">
                            <h3>04. MONITOR QUERIES</h3>
                            <p style={{ color: '#fff' }}>
                                Query logging disabled by default. When enabled, set retention period
                                and review blocked and allowed requests by device. Download logs for
                                analysis.
                            </p>
                        </div>
                    </div>
                </section>

                {/* TECHNICAL SPECIFICATIONS */}
                <section className="window" style={{ marginTop: '20px' }}>
                    <div className="window-title">[ TECHNICAL_SPECIFICATIONS ]</div>
                    <div className="specs-grid">
                        <div className="spec-col">
                            <h3 style={{ marginBottom: '1rem', fontSize: '1.2rem' }}>// DNS_PROTOCOLS</h3>
                            <ul>
                                <li><span>DNS-over-HTTPS (DoH)</span><span>Port 443</span></li>
                                <li><span>DNS-over-TLS (DoT)</span><span>Port 853</span></li>
                                <li><span>DNS-over-QUIC (DoQ)</span><span>Port 853</span></li>
                                <li><span>DNSSEC</span><span>Enabled</span></li>
                            </ul>
                        </div>
                        <div className="spec-col">
                            <h3 style={{ marginBottom: '1rem', fontSize: '1.2rem' }}>// PLATFORM_SUPPORT</h3>
                            <ul>
                                <li><span>System-wide</span><span>Win/Mac/Linux/iOS/Android</span></li>
                                <li><span>Browser</span><span>All supported</span></li>
                                <li><span>IVPN Apps</span><span>Custom DNS</span></li>
                                <li><span>Router/Firewall</span><span>Supported</span></li>
                            </ul>
                        </div>
                        <div className="spec-col">
                            <h3 style={{ marginBottom: '1rem', fontSize: '1.2rem' }}>// PRIVACY_ARCHITECTURE</h3>
                            <ul>
                                <li><span>IP Logging</span><span>None</span></li>
                                <li><span>Query Logging</span><span>Off by Default</span></li>
                                <li><span>Device ID</span><span>Optional</span></li>
                                <li><span>Retention</span><span>Optional (1H-30D)</span></li>
                            </ul>
                        </div>
                    </div>
                </section>

                {/* VERIFIABLE PRIVACY */}
                <section>
                    <div className="section-title">TRUST_SIG</div>
                    <h2 style={{ fontSize: '2.5rem', marginBottom: '1.5rem' }}>Verifiable Privacy</h2>
                    <div className="trust-grid">
                        <div className="window trust-box">
                            <h3>ACCOUNTABLE OPERATORS</h3>
                            <p style={{ color: '#fff' }}>
                                Built by the public team behind IVPN, with a 15-year history in
                                operating privacy services.
                            </p>
                            <a
                                href={LINKS.ivpnTeam}
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                [ MEET THE TEAM ]
                            </a>
                        </div>
                        <div className="window trust-box">
                            <h3>OPEN SOURCE</h3>
                            <p style={{ color: '#fff' }}>
                                The entire modDNS project is open-source. Our implementation is public
                                and available for review.
                            </p>
                            <a
                                href={LINKS.moddnsRepo}
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                [ REVIEW CODE ]
                            </a>
                        </div>
                        <div className="window trust-box">
                            <h3>SECURITY AUDIT</h3>
                            <p style={{ color: '#fff' }}>
                                Independently audited by Cure53 in 2025. Full report available to
                                review.
                            </p>
                            <a
                                href={LINKS.auditReport}
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                [ READ THE REPORT ]
                            </a>
                        </div>
                        <div className="window trust-box">
                            <h3>NO TRACKING</h3>
                            <p style={{ color: '#fff' }}>
                                By default we do not log DNS queries, timestamps, IP addresses and
                                device identifiers.
                            </p>
                            <Link to="/privacy">[ REVIEW OUR POLICIES ]</Link>
                        </div>
                    </div>
                </section>

                {/* SERVICE LIMITATIONS */}
                <section
                    className="window window-sealed"
                    style={{ borderColor: 'var(--alert)', marginTop: '20px' }}
                >
                    <div className="window-title" style={{ color: 'var(--alert)' }}>
                        [ SERVICE_LIMITATIONS ]
                    </div>
                    <ul className="constraints-list">
                        <li>
                            modDNS is a DNS resolver, not a comprehensive privacy solution. It filters
                            DNS queries but does not encrypt other network traffic.
                        </li>
                        <li>
                            Aggressive blocklists may break legitimate services. Start with Basic
                            protection and adjust as needed.
                        </li>
                        <li>
                            Query logging is off by default. Enable only if you need visibility for
                            troubleshooting.
                        </li>
                        <li>
                            Not designed for protection against targeted surveillance or advanced
                            persistent threats.
                        </li>
                        <li>
                            Blocklists update every 1-3 hours. We can't guarantee new malicious domains
                            are blocked immediately.
                        </li>
                    </ul>
                </section>

                {/* UNLINKED ACCESS */}
                <section className="suite-section">
                    <div>
                        <div className="section-title">UNLINKED_SVC</div>
                        <h2 style={{ fontSize: '2.5rem', marginBottom: '1rem' }}>UNLINKED ACCESS</h2>
                        <p style={{ marginBottom: '1.5rem' }}>
                            Additional services in the IVPN privacy stack do not receive or store your
                            IVPN account ID. There is no shared identity layer connecting your accounts
                            across services.
                        </p>
                        <ul>
                            <li style={{ marginBottom: '0.5rem', color: '#fff' }}>
                                Subscription access is verified through token-derived hashes, not
                                account identifiers
                            </li>
                            <li style={{ marginBottom: '0.5rem', color: '#fff' }}>
                                Ongoing subscription sync requires no knowledge of which IVPN account
                                you hold
                            </li>
                            <li style={{ marginBottom: '0.5rem', color: '#fff' }}>
                                Does not prevent all forms of cross-service correlation — see
                                documentation for the full threat model
                            </li>
                        </ul>
                        <p style={{ marginTop: '1.5rem', color: '#aaa', fontSize: '13px' }}>
                            {/* TODO(landing): wrap "Read more about Unlinked Access" in
                                <a href="https://ivpn.net/unlinked-access" target="_blank" rel="noopener noreferrer">…</a>
                                once the IVPN explainer page is published. Until then the prose stays
                                unlinked and only the source-code link is active. */}
                            Read more about Unlinked Access and review the{' '}
                            <a
                                href={LINKS.unlinkedRepo}
                                target="_blank"
                                rel="noopener noreferrer"
                                style={{ color: '#aaa', textDecoration: 'underline' }}
                            >
                                code
                            </a>
                            .
                        </p>
                    </div>
                    <div className="vector-diagram">
                        <span className="meta-data meta-tr">SVC_TOPOLOGY</span>
                        <TopologyDiagram />
                    </div>
                </section>

                {/* GET ACCESS */}
                <section>
                    <div className="section-title">PLAN_INIT</div>
                    <h2 style={{ fontSize: '2.5rem', marginBottom: '1rem' }}>GET ACCESS TO MODDNS</h2>
                    <p style={{ marginBottom: '2rem' }}>
                        modDNS is included in IVPN Plus and Pro Suite. No standalone plan is available.
                        Visit{' '}
                        <a
                            href={LINKS.ivpnHome}
                            target="_blank"
                            rel="noopener noreferrer"
                            style={{ color: 'var(--phosphor)' }}
                        >
                            ivpn.net
                        </a>{' '}
                        for pricing and account setup.
                    </p>
                    <div className="pricing-grid">
                        <div className="plan-box">
                            <div className="plan-header">
                                <div>
                                    <div className="plan-name">IVPN_PLUS</div>
                                    <div className="plan-price">$80<span>/YEAR</span></div>
                                </div>
                                <a
                                    href={LINKS.pricing}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="btn"
                                >
                                    [ START ]
                                </a>
                            </div>
                            <hr className="plan-divider" />
                            <ul>
                                <li>IVPN / 5 Devices</li>
                                <li>Mailx</li>
                                <li>modDNS</li>
                            </ul>
                        </div>
                        <div className="plan-box">
                            <div className="plan-header">
                                <div>
                                    <div className="plan-name">IVPN_PRO_SUITE</div>
                                    <div className="plan-price">$100<span>/YEAR</span></div>
                                </div>
                                <a
                                    href={LINKS.pricing}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="btn"
                                >
                                    [ START ]
                                </a>
                            </div>
                            <hr className="plan-divider" />
                            <ul>
                                <li>IVPN / 10 Devices</li>
                                <li>Mailx</li>
                                <li>modDNS</li>
                                <li>Portmaster Pro</li>
                            </ul>
                        </div>
                    </div>
                </section>

                {/* FOOTER */}
                <footer
                    style={{
                        textAlign: 'center',
                        borderTop: '1px solid var(--phosphor-dim)',
                        paddingTop: '1rem',
                        color: 'var(--phosphor)',
                        fontSize: '12px',
                        marginTop: '2rem',
                    }}
                >
                    modDNS ::{' '}
                    {isAuthenticated ? (
                        <Link to="/home" style={{ color: 'inherit', textDecoration: 'none' }}>DASHBOARD</Link>
                    ) : (
                        <Link to="/login" style={{ color: 'inherit', textDecoration: 'none' }}>LOGIN</Link>
                    )}
                    {' '}::{' '}
                    <Link to="/faq" style={{ color: 'inherit', textDecoration: 'none' }}>FAQ</Link>
                    {' '}::{' '}
                    <Link to="/privacy" style={{ color: 'inherit', textDecoration: 'none' }}>PRIVACY</Link>
                    {' '}:: EOF. CONNECTION TERMINATED.
                </footer>

            </div>
        </div>
    );
}
