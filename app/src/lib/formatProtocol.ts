// The proxy logs DoH over HTTP/3 as `doh3`; `https` covers HTTP/1.1 and HTTP/2,
// which it cannot tell apart behind nginx, so only HTTP/3 gets a detail note.
const LABELS: Record<string, { row: string; detail: string }> = {
    doh3: { row: 'HTTPS', detail: 'HTTPS (HTTP/3)' },
};

const labels = (protocol: string | undefined) => {
    if (!protocol) return { row: '-', detail: '-' };
    if (Object.prototype.hasOwnProperty.call(LABELS, protocol)) return LABELS[protocol];
    const upper = protocol.toUpperCase();
    return { row: upper, detail: upper };
};

export const formatProtocol = (protocol: string | undefined): string => labels(protocol).row;

export const formatProtocolDetail = (protocol: string | undefined): string => labels(protocol).detail;
