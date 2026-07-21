/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

interface ImportMetaEnv {
    readonly VITE_IVPN_HOME_URL: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}
