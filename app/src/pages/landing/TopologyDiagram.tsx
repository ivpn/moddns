// Unlinked Access topology SVG — ported verbatim from the marketing handover HTML.
// Filter IDs (#gn2, #rd2) are namespaced to avoid clashing with any future
// hero-flow SVG that might use #gn / #rd.
export default function TopologyDiagram() {
    return (
        <svg viewBox="0 -8 450 300" xmlns="http://www.w3.org/2000/svg">
            <defs>
                <filter id="gn2" x="-25%" y="-25%" width="150%" height="150%">
                    <feGaussianBlur in="SourceAlpha" stdDeviation="1.5" result="blur" />
                    <feFlood floodColor="#4AF6C3" floodOpacity="0.55" result="color" />
                    <feComposite in="color" in2="blur" operator="in" result="glow" />
                    <feMerge>
                        <feMergeNode in="glow" />
                        <feMergeNode in="SourceGraphic" />
                    </feMerge>
                </filter>
                <filter id="rd2" x="-25%" y="-25%" width="150%" height="150%">
                    <feGaussianBlur in="SourceAlpha" stdDeviation="1.5" result="blur" />
                    <feFlood floodColor="#FF3366" floodOpacity="0.55" result="color" />
                    <feComposite in="color" in2="blur" operator="in" result="glow" />
                    <feMerge>
                        <feMergeNode in="glow" />
                        <feMergeNode in="SourceGraphic" />
                    </feMerge>
                </filter>
            </defs>

            {/* ═══ GREEN GROUP ═══ */}
            <g>
                <rect className="node" x="10" y="121" width="72" height="39" />
                <text x="46" y="145" fill="var(--phosphor)" fontFamily="monospace" fontSize="11" textAnchor="middle">User</text>

                <path
                    d="M 82 140 L 125 140"
                    fill="none"
                    stroke="var(--phosphor)"
                    strokeWidth="1"
                    strokeDasharray="10 10"
                    pathLength="100"
                    style={{ animation: 'dash 180s linear infinite' }}
                />

                <rect className="node" x="125" y="121" width="72" height="39" />
                <text x="161" y="145" fill="var(--phosphor)" fontFamily="monospace" fontSize="11" textAnchor="middle">IVPN</text>

                <path
                    d="M 197 140 L 239 140"
                    fill="none"
                    stroke="var(--phosphor)"
                    strokeWidth="1"
                    strokeDasharray="10 10"
                    pathLength="100"
                    style={{ animation: 'dash 180s linear infinite' }}
                />

                <rect className="node" x="360" y="54" width="77" height="33" />
                <text x="399" y="75" fill="var(--phosphor)" fontFamily="monospace" fontSize="10" textAnchor="middle">modDNS</text>

                <rect className="node" x="360" y="124" width="77" height="33" />
                <text x="399" y="145" fill="var(--phosphor)" fontFamily="monospace" fontSize="10" textAnchor="middle">Mailx</text>

                <rect className="node" x="360" y="194" width="77" height="33" />
                <text x="399" y="215" fill="var(--phosphor)" fontFamily="monospace" fontSize="9" textAnchor="middle">Portmaster</text>
            </g>

            {/* ═══ UA GROUP ═══ */}
            <g>
                <rect
                    className="node"
                    x="239"
                    y="121"
                    width="55"
                    height="39"
                    style={{ strokeWidth: 1.5, stroke: '#fff' }}
                />
                <text x="266" y="145" fill="#fff" fontFamily="monospace" fontSize="11" textAnchor="middle">UA</text>

                <path
                    d="M 294 140 L 360 70"
                    fill="none"
                    stroke="var(--phosphor)"
                    strokeWidth="1"
                    strokeDasharray="10 10"
                    pathLength="100"
                    style={{ animation: 'dash 180s linear infinite' }}
                />
                <path
                    d="M 294 140 L 360 140"
                    fill="none"
                    stroke="var(--phosphor)"
                    strokeWidth="1"
                    strokeDasharray="10 10"
                    pathLength="100"
                    style={{ animation: 'dash 180s linear infinite' }}
                />
                <path
                    d="M 294 140 L 360 210"
                    fill="none"
                    stroke="var(--phosphor)"
                    strokeWidth="1"
                    strokeDasharray="10 10"
                    pathLength="100"
                    style={{ animation: 'dash 180s linear infinite' }}
                />
            </g>
        </svg>
    );
}
