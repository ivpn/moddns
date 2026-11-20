"""Software WebAuthn authenticator for the passkey tests (custom).

Produces the JSON a browser returns from ``navigator.credentials.create()`` and
``navigator.credentials.get()`` (W3C WebAuthn Level 3 §5.1), signed with ES256 keys
the test holds. The byte encodings (authenticator data, attestation object, client
data, COSE key) come from Yubico's ``fido2`` library; only the authenticator
behaviour is ours:

- ``none`` attestation (§8.7) and one ES256 credential per ``create()`` call;
- authenticator data flags set per instance: UP, UV, BE, BS (§6.1);
- one signature counter per credential, raised on every assertion unless the test
  sets it (§6.1.1);
- ``excludeCredentials`` honoured like a browser: creating a credential when the
  authenticator already holds an excluded one raises ``InvalidStateError`` (§6.3.2 step 3).
"""

import hashlib
import os
from dataclasses import dataclass, field

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from fido2 import webauthn
from fido2.cose import ES256

FLAG = webauthn.AuthenticatorData.FLAG


class InvalidStateError(Exception):
    """The authenticator already holds a credential listed in excludeCredentials."""


@dataclass
class Credential:
    id: bytes
    key: ec.EllipticCurvePrivateKey
    user_handle: bytes
    rp_id: str
    sign_count: int = 0


@dataclass
class SoftAuthenticator:
    """An authenticator plus the browser that talks to it, bound to one origin."""

    origin: str
    user_verified: bool = True
    user_present: bool = True
    backup_eligible: bool = False
    backup_state: bool = False
    credentials: dict[bytes, Credential] = field(default_factory=dict)

    def _flags(self, extra: int = 0) -> FLAG:
        flags = FLAG(extra)
        if self.user_present:
            flags |= FLAG.UP
        if self.user_verified:
            flags |= FLAG.UV
        if self.backup_eligible:
            flags |= FLAG.BE
        if self.backup_state:
            flags |= FLAG.BS
        return flags

    def create(self, options: dict) -> tuple[bytes, dict]:
        """Answer creation options; returns the new credential id and the response JSON."""
        opts = webauthn.PublicKeyCredentialCreationOptions.from_dict(options["publicKey"])
        if any(c.id in self.credentials for c in opts.exclude_credentials or []):
            raise InvalidStateError("authenticator already holds an excluded credential")

        key = ec.generate_private_key(ec.SECP256R1())
        cred = Credential(id=os.urandom(32), key=key, user_handle=opts.user.id, rp_id=opts.rp.id)
        self.credentials[cred.id] = cred

        client_data = webauthn.CollectedClientData.create("webauthn.create", opts.challenge, self.origin)
        attested = webauthn.AttestedCredentialData.create(
            b"\x00" * 16, cred.id, ES256.from_cryptography_key(key.public_key())
        )
        auth_data = webauthn.AuthenticatorData.create(
            hashlib.sha256(cred.rp_id.encode()).digest(), self._flags(FLAG.AT), cred.sign_count, attested
        )
        response = webauthn.RegistrationResponse(
            raw_id=cred.id,
            response=webauthn.AuthenticatorAttestationResponse(
                client_data=client_data,
                attestation_object=webauthn.AttestationObject.create("none", auth_data, {}),
            ),
        )
        return cred.id, dict(response)

    def add_credential(self, rp_id: str, user_handle: bytes) -> bytes:
        """Hold a credential no relying party has registered; returns its id."""
        cred = Credential(id=os.urandom(32), key=ec.generate_private_key(ec.SECP256R1()),
                          user_handle=user_handle, rp_id=rp_id)
        self.credentials[cred.id] = cred
        return cred.id

    def get(self, options: dict, credential_id: bytes, *, sign_count: int | None = None) -> dict:
        """Answer request options with the given credential; returns the response JSON.

        ``sign_count`` overrides the counter for this assertion; by default it rises by one.
        """
        opts = webauthn.PublicKeyCredentialRequestOptions.from_dict(options["publicKey"])
        cred = self.credentials[credential_id]
        cred.sign_count = cred.sign_count + 1 if sign_count is None else sign_count

        client_data = webauthn.CollectedClientData.create("webauthn.get", opts.challenge, self.origin)
        auth_data = webauthn.AuthenticatorData.create(
            hashlib.sha256((opts.rp_id or cred.rp_id).encode()).digest(), self._flags(), cred.sign_count
        )
        signature = cred.key.sign(bytes(auth_data) + client_data.hash, ec.ECDSA(hashes.SHA256()))
        response = webauthn.AuthenticationResponse(
            raw_id=cred.id,
            response=webauthn.AuthenticatorAssertionResponse(
                client_data=client_data,
                authenticator_data=auth_data,
                signature=signature,
                user_handle=cred.user_handle,
            ),
        )
        return dict(response)
