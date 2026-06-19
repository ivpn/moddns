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
        ...(input.profileIds !== undefined && { profile_ids: input.profileIds }),
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

      // The backend caps each profile's custom rules in the export (oldest-first)
      // and reports how many profiles were trimmed. Warn the user so they know the
      // file isn't a complete copy. (The limit is 1000 rules per profile.)
      const truncatedHeader = (response.headers as Record<string, string>)['x-moddns-export-truncated'];
      const truncatedCount = truncatedHeader ? parseInt(truncatedHeader, 10) : 0;
      if (truncatedCount > 0) {
        toast.warning(
          `${truncatedCount} profile${truncatedCount === 1 ? '' : 's'} had more than 1000 custom rules — only the first 1000 per profile were exported.`,
        );
      }
    } catch (error) {
      const axiosError = error as { response?: { data?: unknown } };

      // The request uses responseType:'blob', so even error bodies arrive as Blobs.
      // Parse the Blob back to JSON to extract the server error message.
      let serverMessage: string | undefined;
      try {
        const rawData = axiosError.response?.data;
        if (rawData instanceof Blob) {
          const text = await rawData.text();
          const parsed = JSON.parse(text) as { error?: string };
          serverMessage = parsed.error;
        } else if (rawData && typeof rawData === 'object') {
          serverMessage = (rawData as { error?: string }).error;
        }
      } catch {
        // Non-JSON blob — fall through to the generic message
      }

      toast.error(serverMessage ?? 'Export failed. Please try again.');
      throw error;
    } finally {
      setIsExporting(false);
    }
  }, []);

  return { exportProfiles, isExporting };
}
