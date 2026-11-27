import { useState, useCallback } from 'react';
import { toast } from 'sonner';
import API from '@/api/api';
import type { RequestsExportRequest, RequestsExportRequestScopeEnum } from '@/api/client/api';

export type ExportScope = 'all' | 'selected';

export interface ExportProfilesInput {
  scope: ExportScope;
  profileIds?: string[];
  /** Returns the reauth token for profile_export (see useIdentityVerification). */
  reauthToken: () => Promise<string>;
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
        reauth_token: await input.reauthToken(),
      };

      const response = await API.Client.profilesApi.apiV1ProfilesExportPost(body, { responseType: 'blob' });

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
