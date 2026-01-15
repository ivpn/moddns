// /src/guides/windows.ts
export const windowsBadges = [
  { label: "Windows" },
  { label: "DNS over HTTPS" },
];

// Copy-to-clipboard code block (mirrors Browsers guide styling)
import CodeBlock from '@/components/setup/CodeBlock';

// Using shared CodeBlock component

interface WindowsGuideDeps {
  dohEndpoint: string; // fully constructed DoH URL with profile id
  primaryIp: string;
}

export const createWindowsSteps = (deps: WindowsGuideDeps) => {
  const { dohEndpoint, primaryIp } = deps;
  const doh = dohEndpoint;
  return [
    {
      step: 1,
      instruction: (
        <span>
          Go to Windows <span className="font-medium">Settings &gt; Network &amp; internet</span>
        </span>
      ),
    },
    {
      step: 2,
      instruction: (
        <span>
          Click <span className="font-medium">Wi-Fi &gt; Hardware properties</span>, or click <span className="font-medium">Ethernet</span>
        </span>
      ),
    },
    {
      step: 3,
      instruction: (
        <span>
          Click the <span className="font-medium">Edit</span> button beside <span className="font-medium">DNS server assignment</span>
        </span>
      ),
    },
    {
      step: 4,
      instruction: (
        <span>
          Select <span className="font-medium">Manual</span> and toggle <span className="font-medium">IPv4</span> to <span className="font-medium">On</span>
        </span>
      ),
    },
    {
      step: 5,
      instruction: (
        <span>
          In the <span className="font-medium">Preferred DNS</span> field, enter the <CodeBlock inline noWrap value={primaryIp} /> IP address
        </span>
      ),
    },
    {
      step: 6,
      instruction: (
        <span>
          Toggle <span className="font-medium">DNS over HTTPS</span> to <span className="font-medium">On (manual template)</span>
        </span>
      ),
    },
    {
      step: 7,
      instruction: (
        <span>
          Add your DNS query URI  to the <span className="font-medium">DNS over HTTPS template</span> field:
          <br />
          <CodeBlock value={doh} />
        </span>
      ),
    },
    {
      step: 8,
      instruction: (
        <span>
          Click <span className="font-medium">Save</span>
        </span>
      ),
    },
  ];
};

// No static steps exported now; consumer (RightPanelGuide) must inject deps.
// Provide a convenience factory should some part of app want a one-off eager object.
export const buildWindowsGuide = (deps: WindowsGuideDeps) => ({
  badges: windowsBadges,
  steps: createWindowsSteps(deps)
});

export default {
  badges: windowsBadges,
  createWindowsSteps,
  buildWindowsGuide,
};