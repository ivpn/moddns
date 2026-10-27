import { type JSX } from "react";

interface SettingsSectionProps {
    name?: string;
    description?: string;
    /** Tailwind text size class, e.g. 'text-xl', 'text-2xl', 'text-lg' */
    nameSize?: string;
    nameColor?: string;
    descriptionColor?: string;
}

export const SettingsSection = ({
    name = "Settings",
    description = "",
    nameSize = "text-xl",
    nameColor = "text-[var(--tailwind-colors-slate-50)]",
    descriptionColor = "text-[var(--tailwind-colors-slate-100)]",
}: SettingsSectionProps): JSX.Element => {
    return (
        <div className="flex flex-col items-start gap-5 w-full">
            <div className="flex flex-col gap-1 w-full">
                <h2
                    className={`font-bold ${nameColor} ${nameSize} tracking-[-0.60px] leading-7 font-['Roboto_Mono-Bold',Helvetica]`}
                >
                    {name}
                </h2>
                <p className={`${descriptionColor} text-sm leading-5 font-['Roboto_Flex-Regular',Helvetica]`}>
                    {description}
                </p>
            </div>
        </div>
    );
};

export default SettingsSection;