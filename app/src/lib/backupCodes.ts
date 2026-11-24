// The API ignores hyphens and whitespace in a 2FA code, so a code entered in
// this grouped form is accepted as shown.
export function formatBackupCode(code: string): string {
    return code.match(/.{1,5}/g)?.join('-') ?? '';
}
