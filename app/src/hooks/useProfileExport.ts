import { useState, useCallback } from 'react';
import { toast } from 'sonner';
import API from '@/api/api';
import type { RequestsExportRequest, RequestsExportRequestScopeEnum } from '@/api/client/api';

export type ExportScope = 'all' | 'selected';

export interface ExportProfilesInput {
  scope: ExportScope;
  profileIds?: string[];
  currentPassword?: string;
  reauthToken?: string;
  /** 6-digit TOTP code — forwarded only when password method is used and 2FA is enabled. */
  otp?: string;
}

export function useProfileExport(): {
  exportProfiles: (input: ExportProfilesInput) => Promise<void>;
  isExporting: boolean;
} {
  const [isExporting, setIsExporting] = useState(false);

  const exportProfiles = useCallback(async (input: ExportProfilesInput): Promise<void> => {
    setIsExporting(true);
    try {
      const body: RequestsExportRequest = {
        scope: input.scope as RequestsExportRequestScopeEnum,
        ...(input.profileIds !== undefined && { profileIds: input.profileIds }),
        ...(input.currentPassword !== undefined && { current_password: input.currentPassword }),
        ...(input.reauthToken !== undefined && { reauth_token: input.reauthToken }),
      };

      // Forward OTP as x-mfa-code header only when password method is active and a code is provided.
      // Passkey reauth already satisfies the second factor — no OTP needed in that path.
      const axiosOptions: import('axios').RawAxiosRequestConfig = { responseType: 'blob' };
      if (input.currentPassword !== undefined && input.otp) {
        axiosOptions.headers = { 'x-mfa-code': input.otp };
      }

      const response = await API.Client.profilesApi.apiV1ProfilesExportPost(body, axiosOptions);

      const blob = response.data instanceof Blob
        ? response.data
        : new Blob([JSON.stringify(response.data)], { type: 'application/vnd.moddns.export+json' });

      const disposition = (response.headers as Record<string, string>)['content-disposition'] ?? '';
      const filenameMatch = disposition.match(/filename="?([^";\s]+)"?/);
      const filename = filenameMatch?.[1] ?? `moddns-export-${new Date().toISOString().replace(/[:.]/g, '-')}.moddns.json`;

      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', filename);
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (error) {
      const axiosError = error as { response?: { data?: { error?: string } } };
      const message = axiosError.response?.data?.error ?? 'Export failed. Please try again.';
      toast.error(message);
      throw error;
    } finally {
      setIsExporting(false);
    }
  }, []);

  return { exportProfiles, isExporting };
}
