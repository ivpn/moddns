// Reusable WebAuthn stubs for Playwright tests
// Provides deterministic credential responses without relying on real platform authenticators.

/** Install a success stub for navigator.credentials.get returning a minimal public-key assertion */
export async function installWebAuthnSuccessStub(page: import('@playwright/test').Page) {
  await page.addInitScript(() => {
    const enc = new TextEncoder();
    function buf(str: string) { return enc.encode(str).buffer; }
    // @ts-expect-error - mocking WebAuthn API
    navigator.credentials = navigator.credentials || {};
    // @ts-expect-error - mocking WebAuthn API
    navigator.credentials.get = async () => ({
      id: 'cred1',
      rawId: buf('rawId'),
      response: {
        clientDataJSON: buf('clientData'),
        authenticatorData: buf('authData'),
        signature: buf('sig'),
        userHandle: buf('user'),
      },
      type: 'public-key'
    });
  });
}

/** Install a failing stub causing navigator.credentials.get to throw */
export async function installWebAuthnErrorStub(page: import('@playwright/test').Page, message = 'Simulated passkey failure') {
  await page.addInitScript((msg: string) => {
    // @ts-expect-error - mocking WebAuthn API
    navigator.credentials = navigator.credentials || {};
    // @ts-expect-error - mocking WebAuthn API
    navigator.credentials.get = async () => { throw new Error(msg); };
  }, message);
}
