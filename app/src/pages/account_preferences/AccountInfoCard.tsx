import React from "react";
import { Card, CardContent } from "@/components/ui/card";

interface AccountInfoItem {
    label: string;
    // Allow value to be string or any React node (e.g., badge component)
    value: React.ReactNode;
    icon?: React.ReactNode;
}

interface AccountInfoCardProps {
    accountInfo: AccountInfoItem[];
}

const AccountInfoCard: React.FC<AccountInfoCardProps> = ({ accountInfo }) => (
    <Card className="w-full max-w-full bg-transparent dark:bg-[var(--variable-collection-surface)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent overflow-hidden">
        <CardContent className="flex flex-col gap-6 w-full max-w-full">
            <div className="flex flex-col gap-0.5">
                <h2 className="font-mono font-bold text-[var(--tailwind-colors-slate-50)] text-xl leading-6">
                    Account info
                </h2>
            </div>

            <div className="flex flex-col gap-3">
                {accountInfo.map((item, index) => (
                    <div
                        key={index}
                        className="flex flex-col sm:flex-row sm:items-center sm:justify-between w-full gap-1 sm:gap-3 break-words min-w-0"
                    >
                        <span className="font-['Figtree',Helvetica] font-normal text-[var(--tailwind-colors-slate-100)] text-sm leading-[20px] break-words min-w-0">
                            {item.label}
                        </span>
                        <div className="flex items-center justify-start sm:justify-end gap-2 flex-wrap max-w-full break-words min-w-0">
                            {item.icon && (
                                <div className="flex items-center gap-1 flex-shrink-0">
                                    {item.icon}
                                </div>
                            )}
                            {typeof item.value === 'string' ? (
                                item.value !== '' && (
                                    <span className="text-[var(--tailwind-colors-slate-50)] text-sm leading-5 break-words break-all min-w-0">
                                        {item.value}
                                    </span>
                                )
                            ) : (
                                item.value
                            )}
                        </div>
                    </div>
                ))}
            </div>
        </CardContent>
    </Card>
);

export default AccountInfoCard;
