import { type JSX, useEffect, useMemo, useState } from "react";
import { ScrollArea } from "@/components/ui/scroll-area";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import { toast } from "sonner";
import ServiceCard from "@/pages/blocklists/ServiceCard";
import { getServiceLogoSrc } from "@/assets/services";
import type {
    ApiServicesUpdates,
    ServicescatalogService,
} from "@/api/client/api";

function formatASNs(asns?: Array<number>): string {
    if (!asns || asns.length === 0) return "No ASNs";
    const shown = asns.slice(0, 5).join(", ");
    if (asns.length <= 5) return `ASNs: ${shown}`;
    return `ASNs: ${shown} +${asns.length - 5}`;
}

function formatASNsTitle(asns?: Array<number>): string {
    if (!asns || asns.length === 0) return "No ASNs";
    return `ASNs: ${asns.join(", ")}`;
}

export default function ServicesContentSection(): JSX.Element {
    const activeProfile = useAppStore((state) => state.activeProfile);
    const setActiveProfile = useAppStore((state) => state.setActiveProfile);

    const [services, setServices] = useState<ServicescatalogService[]>([]);
    const [loading, setLoading] = useState(true);
    const [updating, setUpdating] = useState<string | null>(null);

    const blockedServices = activeProfile?.settings?.privacy?.services?.blocked ?? [];

    useEffect(() => {
        let isActive = true;

        const fetchCatalog = async () => {
            setLoading(true);
            try {
                const resp = await api.Client.servicesApi.apiV1ServicesGet();
                if (!isActive) return;
                setServices(resp.data?.services ?? []);
            } catch {
                if (!isActive) return;
                setServices([]);
            } finally {
                if (isActive) setLoading(false);
            }
        };

        fetchCatalog();
        return () => {
            isActive = false;
        };
    }, []);

    const serviceById = useMemo(() => {
        const map = new Map<string, ServicescatalogService>();
        for (const svc of services) {
            if (svc.id) map.set(svc.id, svc);
        }
        return map;
    }, [services]);

    const handleServiceSwitch = async (serviceId: string, checked: boolean) => {
        if (!activeProfile?.profile_id) return;
        if (!serviceId) return;

        setUpdating(serviceId);
        try {
            const payload: ApiServicesUpdates = { service_ids: [serviceId] };

            if (checked) {
                await api.Client.profilesApi.apiV1ProfilesIdServicesPost(activeProfile.profile_id, payload);
            } else {
                await api.Client.profilesApi.apiV1ProfilesIdServicesDelete(activeProfile.profile_id, payload);
            }

            const updatedProfile = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updatedProfile.data);

            const svcName = serviceById.get(serviceId)?.name || serviceId;
            toast.success(checked ? "Service blocked" : "Service unblocked", {
                description: checked
                    ? `${svcName} has been blocked successfully.`
                    : `${svcName} has been unblocked successfully.`,
            });
        } catch {
            toast.error("Error", {
                description: "Failed to update service. Please try again.",
            });
        } finally {
            setUpdating(null);
        }
    };

    return (
        <div className="flex flex-col w-full items-start gap-6">
            <section className="w-full">
                <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                    Services are ASN-based presets. When enabled, modDNS blocks DNS answers whose destination IP belongs to the service’s network.
                </p>
            </section>

            <section className="w-full">
                <ScrollArea className="w-full">
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                        {loading ? (
                            <div className="col-span-full text-center text-[var(--tailwind-colors-slate-400)] py-8">
                                Loading services...
                            </div>
                        ) : services.length === 0 ? (
                            <div className="col-span-full text-center text-[var(--tailwind-colors-slate-400)] py-8">
                                No services available.
                            </div>
                        ) : (
                            services.map((svc, idx) => {
                                const id = svc.id ?? "";
                                const name = svc.name ?? "Unnamed";
                                const isBlocked = id ? blockedServices.includes(id) : false;
                                const asnsLabel = formatASNs(svc.asns);
                                const asnsTitle = formatASNsTitle(svc.asns);
                                const logoSrc = getServiceLogoSrc({ serviceId: id, serviceName: name });

                                return (
                                    <ServiceCard
                                        key={id || `${name}-${idx}`}
                                        name={name}
                                        description={`Block ${name} service and all its domains.`}
                                        asnsLabel={asnsLabel}
                                        asnsTitle={asnsTitle}
                                        logoSrc={logoSrc}
                                        onSwitchChange={(checked) => handleServiceSwitch(id, checked)}
                                        switchChecked={isBlocked}
                                        switchDisabled={updating === id || !id}
                                    />
                                );
                            })
                        )}
                    </div>
                </ScrollArea>
            </section>
        </div>
    );
}
