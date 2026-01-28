import React from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Trash2 } from "lucide-react";

interface DeleteProfileSectionProps {
    onDeleteClick: () => void;
}

const DeleteProfileSection: React.FC<DeleteProfileSectionProps> = ({
    onDeleteClick,
}) => (
    <Card className="w-full border-none bg-[var(--danger-zone-bg)] rounded-[var(--primitives-radius-radius)]">
        <CardContent className="bg-transparent">
            <div className="flex flex-col items-start gap-6 w-full">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between w-full gap-4 flex-wrap">
                    <div className="flex flex-col items-start gap-2 min-w-0 max-w-full">
                        <div className="[font-family:'Roboto_Flex-Medium',Helvetica] font-bold text-[var(--shadcn-ui-app-foreground)] text-base tracking-[0] leading-4 break-words">
                            Delete profile
                        </div>
                        <div className="font-text-sm-leading-5-normal font-[number:var(--text-sm-leading-5-normal-font-weight)] text-[var(--shadcn-ui-app-foreground)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)] [font-style:var(--text-sm-leading-5-normal-font-style)] break-words">
                            You can delete your profile immediately, removing all associated settings and data. Account preferences and other profiles are unaffected. Cannot be reversed.
                        </div>
                    </div>

                    <Button
                        className="h-auto min-h-11 lg:min-h-0 flex items-center justify-center px-2 py-1.5 bg-[var(--tailwind-colors-red-600)] rounded-[var(--primitives-radius-radius-md)] gap-1 hover:bg-[var(--tailwind-colors-red-400)] w-full sm:w-auto"
                        onClick={onDeleteClick}
                    >
                        <Trash2 className="w-4 h-4 text-white" />
                        <span className="font-text-sm-leading-6-medium font-[number:var(--text-sm-leading-6-medium-font-weight)] text-white text-[length:var(--text-sm-leading-6-medium-font-size)] tracking-[var(--text-sm-leading-6-medium-letter-spacing)] leading-[var(--text-sm-leading-6-medium-line-height)] break-words [font-style:var(--text-sm-leading-6-medium-font-style)]">Delete profile</span>
                    </Button>
                </div>
            </div>
        </CardContent>
    </Card>
);

export default DeleteProfileSection;
