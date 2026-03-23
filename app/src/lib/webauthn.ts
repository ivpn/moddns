import {
  startRegistration,
  startAuthentication,
  browserSupportsWebAuthn,
  platformAuthenticatorIsAvailable,
} from '@simplewebauthn/browser';
import type {
  RegistrationResponseJSON,
  AuthenticationResponseJSON,
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
} from '@simplewebauthn/browser';
import api from "@/api/api";
import type { ApiWebAuthnRegisterBeginRequest, ApiWebAuthnLoginBeginRequest } from "@/api/client";
import axios from "axios";

/** Shared shape for WebAuthn errors with name, message, and optional Axios-like response */
interface WebAuthnError extends Error {
  response?: { status?: number; data?: { error?: string; message?: string } };
}

/**
 * Utility functions for WebAuthn passkey operations using SimpleWebAuthn
 */

/**
 * Register a new passkey for the user
 */
export async function registerPasskey(email: string, subid: string): Promise<void> {
  try {
    // Check WebAuthn support
    if (!browserSupportsWebAuthn()) {
      throw new Error("WebAuthn is not supported in this browser");
    }

    // Begin WebAuthn registration process
    const beginResponse = await api.Client.authApi.apiV1WebauthnRegisterBeginPost({ 
      email,
      subid,
    } as ApiWebAuthnRegisterBeginRequest);
    
    const options = beginResponse.data;

    if (!options.publicKey) {
      throw new Error("Invalid WebAuthn options received from server");
    }

    // Start registration with SimpleWebAuthn using the public key directly
    let registrationResponse: RegistrationResponseJSON;
    try {
      registrationResponse = await startRegistration({
        optionsJSON: options.publicKey as unknown as PublicKeyCredentialCreationOptionsJSON
      });
    } catch (error: unknown) {
      if (error instanceof Error && error.name === 'InvalidStateError') {
        throw new Error('A passkey might already be registered for this device');
      }
      throw error;
    }

    // Send the registration response to the server
    const finishResponse = await api.Client.authApi.apiV1WebauthnRegisterFinishPost({
      data: JSON.stringify(registrationResponse)
    });

    if (finishResponse.status !== 201) {
      throw new Error("Failed to complete passkey registration");
    }
  } catch (err: unknown) {
    console.error('Passkey registration error:', err);
    const webauthnErr = err as WebAuthnError;

    // Handle 429 rate limiting errors
    if (axios.isAxiosError(err)) {
      if (err.response?.status === 429) {
        throw new Error("Too many requests. Please try again in a moment.");
      }
    }

    if (webauthnErr.name === 'NotAllowedError') {
      throw new Error("Passkey registration was cancelled or timed out");
    } else if (webauthnErr.name === 'InvalidStateError') {
      throw new Error("A passkey is already registered for this account on this device");
    } else if (webauthnErr.name === 'NotSupportedError') {
      throw new Error("Passkey registration is not supported on this device");
    } else if (webauthnErr.response?.data?.error) {
      throw new Error(webauthnErr.response.data.error);
    } else if (webauthnErr.message) {
      throw err;
    } else {
      throw new Error("Passkey registration failed");
    }
  }
}

/**
 * Authenticate using a passkey
 */
export async function authenticateWithPasskey(
  email: string, 
  onSessionLimitReached?: () => void,
  xSessionsRemove?: boolean
): Promise<void> {
  try {
    // Check WebAuthn support
    if (!browserSupportsWebAuthn()) {
      throw new Error("WebAuthn is not supported in this browser");
    }

    // Begin WebAuthn login process
    const beginResponse = await api.Client.authApi.apiV1WebauthnLoginBeginPost({ 
      email 
    } as ApiWebAuthnLoginBeginRequest);
    
    const options = beginResponse.data;

    if (!options.publicKey) {
      throw new Error("Invalid WebAuthn options received from server");
    }

    // Start authentication with SimpleWebAuthn using the public key directly
    let authenticationResponse: AuthenticationResponseJSON;
    try {
      authenticationResponse = await startAuthentication({
        optionsJSON: options.publicKey as unknown as PublicKeyCredentialRequestOptionsJSON
      });
    } catch (error: unknown) {
      if (error instanceof Error && error.name === 'InvalidStateError') {
        throw new Error('No passkey found for this account');
      }
      throw error;
    }

    // Send the authentication response to the server
    const finishResponse = await api.Client.authApi.apiV1WebauthnLoginFinishPost(
      xSessionsRemove ? "true" : undefined, // xSessionsRemove parameter
      {
        data: JSON.stringify(authenticationResponse)
      }
    );

    if (finishResponse.status !== 201) {
      throw new Error("Passkey authentication failed");
    }

    // Authentication successful - this is where the success toast should be shown
  } catch (err: unknown) {
    console.error('Passkey authentication error:', err);
    const webauthnErr = err as WebAuthnError;

    // Handle 429 rate limiting and session limit errors
    if (axios.isAxiosError(err)) {
      if (err.response?.status === 429) {
        // Check for specific session limit error in response data
        const errorMessage = (err.response?.data as Record<string, string>)?.error || (err.response?.data as Record<string, string>)?.message || '';
        if (errorMessage.toLowerCase().includes('maximum number of active sessions reached')) {
          // If callback is provided, trigger session limit dialog instead of throwing error
          if (onSessionLimitReached) {
            onSessionLimitReached();
            return; // Don't throw error, let dialog handle it
          }
          throw new Error("Maximum number of active sessions reached. Please log out from other devices or wait for sessions to expire.");
        }
        // General 429 error
        throw new Error("Too many requests. Please try again in a moment.");
      }
    }

    if (webauthnErr.name === 'NotAllowedError') {
      throw new Error("Passkey authentication was cancelled or timed out");
    } else if (webauthnErr.name === 'InvalidStateError') {
      throw new Error("No passkey found for this account");
    } else if (webauthnErr.name === 'NotSupportedError') {
      throw new Error("Passkey authentication is not supported on this device");
    } else if (webauthnErr.response?.data?.error) {
      throw new Error(webauthnErr.response.data.error);
    } else if (webauthnErr.message) {
      throw err;
    } else {
      throw new Error("Passkey authentication failed");
    }
  }
}

/**
 * Add a new passkey to an authenticated user's account
 */
export async function addPasskeyToAccount(): Promise<void> {
  try {
    // Check WebAuthn support
    if (!browserSupportsWebAuthn()) {
      throw new Error("WebAuthn is not supported in this browser");
    }

    // Begin WebAuthn add passkey process
    const beginResponse = await api.Client.authApi.apiV1WebauthnPasskeyAddBeginPost();
    
    const options = beginResponse.data;

    // Determine the correct options format
    let webauthnOptions: unknown;
    if (options && typeof options === 'object' && 'publicKey' in options) {
      // Response has publicKey wrapper (like regular registration)
      webauthnOptions = (options as Record<string, unknown>).publicKey;
    } else {
      // Response is the options directly
      webauthnOptions = options;
    }

    if (!webauthnOptions) {
      throw new Error("Invalid WebAuthn options received from server");
    }

    // Start registration with SimpleWebAuthn
    let registrationResponse: RegistrationResponseJSON;
    try {
      registrationResponse = await startRegistration({
        optionsJSON: webauthnOptions as PublicKeyCredentialCreationOptionsJSON
      });
    } catch (error: unknown) {
      if (error instanceof Error && error.name === 'InvalidStateError') {
        throw new Error('A passkey might already be registered for this device');
      }
      throw error;
    }

    // Send the registration response to the server
    const finishResponse = await api.Client.authApi.apiV1WebauthnPasskeyAddFinishPost({
      data: JSON.stringify(registrationResponse)
    });

    if (finishResponse.status !== 201) {
      throw new Error("Failed to complete passkey addition");
    }
  } catch (err: unknown) {
    console.error('Add passkey error:', err);
    const webauthnErr = err as WebAuthnError;

    // Handle 429 rate limiting errors
    if (axios.isAxiosError(err)) {
      if (err.response?.status === 429) {
        throw new Error("Too many requests. Please try again in a moment.");
      }
    }

    if (webauthnErr.name === 'NotAllowedError') {
      throw new Error("Passkey addition was cancelled or timed out");
    } else if (webauthnErr.name === 'InvalidStateError') {
      throw new Error("A passkey is already registered for this account on this device");
    } else if (webauthnErr.name === 'NotSupportedError') {
      throw new Error("Passkey addition is not supported on this device");
    } else if (webauthnErr.response?.data?.error) {
      throw new Error(webauthnErr.response.data.error);
    } else if (webauthnErr.message) {
      throw err;
    } else {
      throw new Error("Failed to add passkey");
    }
  }
}

/**
 * Check if WebAuthn is supported in the current browser
 */
export function isWebAuthnSupported(): boolean {
  return browserSupportsWebAuthn();
}

/**
 * Check if platform authenticator (like Touch ID, Face ID, Windows Hello) is available
 */
export async function isPlatformAuthenticatorAvailable(): Promise<boolean> {
  if (!browserSupportsWebAuthn()) {
    return false;
  }
  
  try {
    return await platformAuthenticatorIsAvailable();
  } catch {
    return false;
  }
}

/**
 * Begin and finish passkey-based reauthentication for an email change.
 * Returns a short-lived reauth token consumed by the email update payload.
 */
export async function beginEmailChangeReauth(): Promise<string> {
  try {
    const begin = await api.Client.authApi.apiV1WebauthnPasskeyReauthBeginPost({ purpose: 'email_change' as unknown as string });
    const options = begin.data;
    const optionsRecord = options as Record<string, unknown>;
    const opts = optionsRecord.publicKey ? optionsRecord.publicKey : options;
    const assertionResponse = await startAuthentication({ optionsJSON: opts as PublicKeyCredentialRequestOptionsJSON });
    const finish = await api.Client.authApi.apiV1WebauthnPasskeyReauthFinishPost({ data: JSON.stringify(assertionResponse) });
    const token = finish.data.reauth_token;
    if (!token) throw new Error('Missing reauth token');
    return token;
  } catch (err: unknown) {
    const webauthnErr = err as WebAuthnError;
    if (webauthnErr.name === 'NotAllowedError') {
      throw new Error('Passkey verification was cancelled or timed out.');
    }
    if (webauthnErr.response?.data?.error) {
      throw new Error(webauthnErr.response.data.error);
    }
    throw new Error(webauthnErr.message || 'Passkey reauthentication failed.');
  }
}

/**
 * Begin and finish passkey-based reauthentication for account deletion.
 * Returns a short-lived reauth token consumed by the account deletion payload.
 */
export async function beginAccountDeletionReauth(): Promise<string> {
  try {
    const begin = await api.Client.authApi.apiV1WebauthnPasskeyReauthBeginPost({ purpose: 'account_deletion' as unknown as string });
    const options = begin.data;
    const optionsRecord = options as Record<string, unknown>;
    const opts = optionsRecord.publicKey ? optionsRecord.publicKey : options;
    const assertionResponse = await startAuthentication({ optionsJSON: opts as PublicKeyCredentialRequestOptionsJSON });
    const finish = await api.Client.authApi.apiV1WebauthnPasskeyReauthFinishPost({ data: JSON.stringify(assertionResponse) });
    const token = finish.data.reauth_token;
    if (!token) throw new Error('Missing reauth token');
    return token;
  } catch (err: unknown) {
    const webauthnErr = err as WebAuthnError;
    if (webauthnErr.name === 'NotAllowedError') {
      throw new Error('Passkey verification was cancelled or timed out.');
    }
    if (webauthnErr.response?.data?.error) {
      throw new Error(webauthnErr.response.data.error);
    }
    throw new Error(webauthnErr.message || 'Passkey reauthentication failed.');
  }
}
