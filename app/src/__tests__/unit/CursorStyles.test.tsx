import { describe, it, expect, afterEach } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Checkbox } from "@/components/ui/checkbox";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetTitle,
} from "@/components/ui/sheet";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { RuleComposer } from "@/pages/custom_rules/RuleComposer";

const noop = () => { };

afterEach(() => {
    cleanup();
});

describe("Pointer cursor styles", () => {
    it("buttons expose pointer cursor styles", () => {
        const { rerender } = render(<Button>Click me</Button>);
        const button = screen.getByRole("button", { name: "Click me" });
        expect(button.className).toContain("cursor-pointer");

        rerender(<Button disabled>Disabled</Button>);
        const disabledButton = screen.getByRole("button", { name: "Disabled" });
        expect(disabledButton.className).toContain("cursor-pointer");
        expect(disabledButton.className).toContain("disabled:cursor-not-allowed");
    });

    it("tabs triggers show pointer cursor", () => {
        render(
            <Tabs defaultValue="one">
                <TabsList>
                    <TabsTrigger value="one">One</TabsTrigger>
                </TabsList>
            </Tabs>
        );

        const tab = screen.getByRole("tab", { name: "One" });
        expect(tab.className).toContain("cursor-pointer");
    });

    it("select triggers show pointer cursor", () => {
        render(
            <Select defaultValue="one">
                <SelectTrigger data-testid="select-trigger">
                    <SelectValue placeholder="Pick" />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="one">One</SelectItem>
                </SelectContent>
            </Select>
        );

        const trigger = screen.getByTestId("select-trigger");
        expect(trigger.className).toContain("cursor-pointer");
    });

    it("switches show pointer cursor", () => {
        render(<Switch aria-label="Demo switch" />);
        const switchEl = screen.getByRole("switch");
        expect(switchEl.className).toContain("cursor-pointer");
    });

    it("checkboxes show pointer cursor", () => {
        render(<Checkbox aria-label="Demo checkbox" />);
        const checkbox = screen.getByRole("checkbox");
        expect(checkbox.className).toContain("cursor-pointer");
    });

    it("dialog close buttons show pointer cursor", () => {
        render(
            <Dialog open onOpenChange={noop}>
                <DialogContent>
                    <DialogTitle>Demo dialog</DialogTitle>
                    <DialogDescription>Helper copy</DialogDescription>
                    <p>Dialog body</p>
                </DialogContent>
            </Dialog>
        );

        const closeButton = screen.getByRole("button", { name: /close/i });
        expect(closeButton.className).toContain("cursor-pointer");
    });

    it("sheet close buttons show pointer cursor", () => {
        render(
            <Sheet open onOpenChange={noop}>
                <SheetContent side="right">
                    <SheetHeaderContent />
                    <p>Sheet body</p>
                </SheetContent>
            </Sheet>
        );

        const closeButton = screen.getByRole("button", { name: /close/i });
        expect(closeButton.className).toContain("cursor-pointer");
    });

    it("dropdown menu items show pointer cursor", () => {
        render(
            <DropdownMenu open onOpenChange={noop}>
                <DropdownMenuTrigger>Open</DropdownMenuTrigger>
                <DropdownMenuContent>
                    <DropdownMenuItem>Rename group</DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
        );

        const item = screen.getByRole("menuitem", { name: "Rename group" });
        expect(item.className).toContain("cursor-pointer");
    });

    it("rule composer control shows text cursor", () => {
        const { container } = render(
            <RuleComposer
                tokens={[]}
                onTokensChange={() => { }}
                onSubmit={() => { }}
                loading={false}
                action="denylist"
            />
        );

        const control = container.querySelector(".rule-composer__control");
        expect(control).not.toBeNull();
        expect(control).toHaveStyle({ cursor: "text" });
    });
});

const SheetHeaderContent = () => (
    <>
        <SheetTitle>Demo sheet</SheetTitle>
        <SheetDescription>Helper copy</SheetDescription>
    </>
);
