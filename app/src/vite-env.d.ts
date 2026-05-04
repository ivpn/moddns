/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_IVPN_HOME_URL: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}
