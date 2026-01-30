import { Search } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";

interface EmptyStateProps {
    searchTerm?: string;
}

export default function EmptyState({ searchTerm }: EmptyStateProps) {
    return (
        <Card className="flex flex-col items-start relative flex-1 self-stretch w-full grow bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg overflow-hidden border-0">
            <CardContent className="flex flex-col items-start gap-8 p-4 relative self-stretch w-full">
                <div className="flex flex-col items-center justify-center gap-2.5 relative flex-1 self-stretch w-full grow">
                    <div className="inline-flex flex-col gap-4 flex-[0_0_auto] items-center justify-center relative">
                        <div className="flex w-12 h-12 gap-2.5 rounded-[var(--primitives-radius-radius-sm)] items-center justify-center relative">
                            <Search className="!relative !w-9 !h-9 text-[var(--tailwind-colors-rdns-600)]" />
                        </div>

                        <div className="flex flex-col gap-[var(--tailwind-primitives-gap-gap-2)] self-stretch w-full flex-[0_0_auto] items-center justify-center relative">
                            <div className="relative self-stretch mt-[-1.00px] [font-family:'Roboto_Flex-Bold',Helvetica] font-bold text-transparent text-lg text-center tracking-[0] leading-5">
                                {searchTerm ? (
                                    <>
                                        <span className="text-[var(--tailwind-colors-slate-50)]">No results for &apos;</span>
                                        <span className="text-[var(--tailwind-colors-rdns-600)]">{searchTerm}</span>
                                        <span className="text-[var(--tailwind-colors-slate-50)]">&apos;.</span>
                                    </>
                                ) : (
                                    <span className="text-[var(--tailwind-colors-slate-50)]">No blocklists found.</span>
                                )}
                            </div>

                            <div className="relative w-[390px] text-[var(--tailwind-colors-slate-100)] font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-tailwind-colors-slate-100 text-[length:var(--text-sm-leading-5-normal-font-size)] text-center tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)]">
                                Please try again
                            </div>
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}
