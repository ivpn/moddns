import { useState, useCallback } from 'react';
import { toast } from 'sonner';
import API from '@/api/api';
import type { RequestsImportPayload } from '@/api/client/api';
import { RequestsImportRequestModeEnum } from '@/api/client/api';

export interface ImportProfilesInput {
  payload: RequestsImportPayload;
  currentPassword?: string;
  reauthToken?: string;
  /** 6-digit TOTP code — forwarded only when password method is used and 2FA is enabled. */
  otp?: string;
}

export interface ImportResult {
  createdProfileIds: string[];
  warnings: string[];
}

export function useProfileImport(): {
  importProfiles: (input: ImportProfilesInput) => Promise<ImportResult>;
  isImporting: boolean;
  result: ImportResult | null;
  reset: () => void;
} {
  const [isImporting, setIsImporting] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);

  const importProfiles = useCallback(async (input: ImportProfilesInput): Promise<ImportResult> => {
    setIsImporting(true);
    setResult(null);
    try {
      // Forward OTP as x-mfa-code header only when password method is active and a code is provided.
      // Passkey reauth already satisfies the second factor — no OTP needed in that path.
      const axiosOptions: import('axios').RawAxiosRequestConfig = {};
      if (input.currentPassword !== undefined && input.otp) {
        axiosOptions.headers = { 'x-mfa-code': input.otp };
      }

      const response = await API.Client.profilesApi.apiV1ProfilesImportPost(
        'confirm',
        {
          mode: RequestsImportRequestModeEnum.CreateNew,
          payload: input.payload,
          ...(input.currentPassword !== undefined && { current_password: input.currentPassword }),
          ...(input.reauthToken !== undefined && { reauth_token: input.reauthToken }),
        },
        axiosOptions,
      );

      const importResult: ImportResult = {
        createdProfileIds: response.data.createdProfileIds ?? [],
        warnings: response.data.warnings ?? [],
      };
      setResult(importResult);
      return importResult;
    } catch (error) {
      const axiosError = error as { response?: { data?: { error?: string } } };
      const message = axiosError.response?.data?.error ?? 'Import failed. Please try again.';
      toast.error(message);
      throw error;
    } finally {
      setIsImporting(false);
    }
  }, []);

  const reset = useCallback(() => {
    setResult(null);
  }, []);

  return { importProfiles, isImporting, result, reset };
}
