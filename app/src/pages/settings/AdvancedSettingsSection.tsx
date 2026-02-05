import React from "react";
import ToggleGroup from "@/components/general/ToggleGroup";
import { Card, CardContent } from "@/components/ui/card";
import RecursorChoiceSection from "@/pages/settings/RecursorChoiceSection";

interface AdvancedSettingsSectionProps {
    advancedSettings: { title: string; description: string; options: { value: string; label: string; icon?: string }[]; value: string }[];
    advancedLoading: boolean;
    handleAdvancedChange: (idx: number, value: string) => void;
    currentRecursor: string;
    onRecursorChange: (recursor: string) => void;
}

const AdvancedSettingsSection: React.FC<AdvancedSettingsSectionProps> = ({
    advancedSettings,
    advancedLoading,
    handleAdvancedChange,
    currentRecursor,
    onRecursorChange,
}) => {
    // Check if recursor choice should be visible based on environment
    const shouldShowRecursorChoice = () => {
        const environment = import.meta.env.VITE_SENTRY_ENVIRONMENT;
        return environment === 'local' || environment === 'staging';
    };

    return (
        <Card className="w-full bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
            <CardContent>
                <div className="flex flex-col items-start gap-6 w-full">
                    <div className="flex items-center gap-2 w-full">
                        <div className="flex flex-col items-start gap-2">
                            <div className="[font-family:'Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base tracking-[0] leading-4 break-words">
                                ADVANCED SETTINGS
                            </div>
                            <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] whitespace-nowrap [font-style:var(--text-sm-leading-5-normal-font-style)]">
                                Settings below can significantly affect your service
                                experience. Tread carefully.
                            </div>
                        </div>
                    </div>

                    {advancedSettings.map((setting, index) => (
                        <div
                            key={index}
                            className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-3 sm:gap-4 max-w-full"
                        >
                            <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                                <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--tailwind-colors-slate-50)] text-base tracking-[0] leading-4 break-words">
                                    {setting.title}
                                </div>
                                {setting.description && (
                                    <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--tailwind-colors-slate-200)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">
                                        {setting.description}
                                    </div>
                                )}
                            </div>

                            <ToggleGroup
                                options={setting.options}
                                value={setting.value}
                                onChange={value => handleAdvancedChange(index, value)}
                                variant="outline"
                                className="rounded p-0.5 self-start sm:self-auto"
                            />
                        </div>
                    ))}

                    {/* Conditionally render Recursor Choice Section based on environment */}
                    {shouldShowRecursorChoice() && (
                        <RecursorChoiceSection
                            currentRecursor={currentRecursor}
                            onRecursorChange={onRecursorChange}
                            loading={advancedLoading}
                        />
                    )}
                </div>
            </CardContent>
        </Card>
    );
};

export default AdvancedSettingsSection;