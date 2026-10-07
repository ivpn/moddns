import api from "@/api/api";
import { ModelProfileUpdateOperationEnum, ModelProfileUpdatePathEnum, type ModelProfile } from "@/api/client";
import type { StatsRetention } from "@/components/data-collection/model";

export async function setStatisticsRetention(profileId: string, retention: StatsRetention): Promise<ModelProfile> {
    const res = await api.Client.profilesApi.apiV1ProfilesIdPatch(profileId, {
        updates: [
            {
                operation: ModelProfileUpdateOperationEnum.Replace,
                path: ModelProfileUpdatePathEnum.SettingsStatisticsRetention,
                value: retention as unknown as object,
            },
        ],
    });
    return res.data;
}

/** DELETE /profiles/{id}/statistics (204). */
export async function deleteStatisticsHistory(profileId: string): Promise<void> {
    await api.Client.statisticsApi.apiV1ProfilesIdStatisticsDelete(profileId);
}
