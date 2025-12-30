import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import { RuleComposer } from "@/pages/custom_rules/RuleComposer";

type RuleComposerProps = Parameters<typeof RuleComposer>[0];

const renderRuleComposer = (overrideProps: Partial<RuleComposerProps> = {}) => {
    const props: RuleComposerProps = {
        tokens: [],
        onTokensChange: vi.fn(),
        onSubmit: vi.fn(),
        loading: false,
        action: "denylist",
        ...overrideProps,
    };

    return render(<RuleComposer {...props} />);
};

const getEditableInput = (): HTMLInputElement => {
    const combobox = screen.getByRole("combobox");
    if (combobox instanceof HTMLInputElement) {
        return combobox;
    }

    const input = combobox.querySelector("input");
    if (!input) {
        throw new Error("RuleComposer input element not found");
    }

    return input as HTMLInputElement;
};

describe("RuleComposer", () => {
    it("preserves typed input when focus leaves the field", async () => {
        const user = userEvent.setup();
        renderRuleComposer();

        const input = getEditableInput();
        await user.type(input, "example.com");
        fireEvent.blur(input);

        expect(input).toHaveValue("example.com");
    });

    it("creates a token when separators are typed and clears the field", async () => {
        const user = userEvent.setup();
        const onTokensChange = vi.fn();
        renderRuleComposer({ onTokensChange });

        const input = getEditableInput();
        await user.type(input, "example.com ");

        expect(onTokensChange).toHaveBeenCalledWith([
            { label: "example.com", value: "example.com" },
        ]);
        expect(input).toHaveValue("");
    });

    it("submits tokens when Enter is pressed with an empty input", () => {
        const onSubmit = vi.fn();
        renderRuleComposer({
            onSubmit,
            tokens: [{ label: "example.com", value: "example.com" }],
        });

        const input = getEditableInput();
        fireEvent.keyDown(input, { key: "Enter", code: "Enter" });

        expect(onSubmit).toHaveBeenCalledTimes(1);
    });
});
