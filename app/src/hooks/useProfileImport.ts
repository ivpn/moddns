import { useState, useCallback } from 'react';
import { toast } from 'sonner';
import API from '@/api/api';
import type { ModelExportEnvelope } from '@/api/client/api';
import { RequestsImportRequestModeEnum } from '@/api/client/api';
import { importErrorMessage } from '@/lib/importExportErrors';

export interface ImportProfilesInput {
  payload: ModelExportEnvelope;
  /** Returns the reauth token for profile_import (see useIdentityVerification). */
  reauthToken: () => Promise<string>;
}

export interface ImportResult {
  createdProfileIds: string[];
  /** Resolved profile names in the same order as createdProfileIds. Reflects
   *  any I24 collision rename, so it's the name the user will see in the UI. */
  createdProfileNames: string[];
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
      const token = await input.reauthToken();
      const response = await API.Client.profilesApi.apiV1ProfilesImportPost(
        'confirm',
        {
          mode: RequestsImportRequestModeEnum.CreateNew,
          payload: input.payload,
          reauth_token: token,
        },
      );

      const importResult: ImportResult = {
        createdProfileIds: response.data.createdProfileIds ?? [],
        createdProfileNames: response.data.createdProfileNames ?? [],
        warnings: response.data.warnings ?? [],
      };
      setResult(importResult);
      return importResult;
    } catch (error) {
      const axiosError = error as { response?: { status?: number; data?: { error?: string } } };
      const status = axiosError.response?.status;
      const serverError = axiosError.response?.data?.error;

      toast.error(importErrorMessage(status, serverError));
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
