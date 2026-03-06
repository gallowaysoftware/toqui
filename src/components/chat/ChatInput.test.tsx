import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ChatInput } from "./ChatInput";

describe("ChatInput", () => {
  it("renders a textarea and send button", () => {
    render(<ChatInput onSend={vi.fn()} />);
    expect(screen.getByRole("textbox")).toBeInTheDocument();
    expect(screen.getByRole("button")).toBeInTheDocument();
  });

  it("send button is disabled when input is empty", () => {
    render(<ChatInput onSend={vi.fn()} />);
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("send button is disabled when disabled prop is true", () => {
    render(<ChatInput onSend={vi.fn()} disabled={true} />);
    expect(screen.getByRole("button")).toBeDisabled();
    expect(screen.getByRole("textbox")).toBeDisabled();
  });

  it("calls onSend with trimmed text when button is clicked", async () => {
    const onSend = vi.fn();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "  Hello world  " } });
    fireEvent.click(screen.getByRole("button"));

    expect(onSend).toHaveBeenCalledWith("Hello world");
  });

  it("clears input after sending", () => {
    const onSend = vi.fn();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button"));

    expect((textarea as HTMLTextAreaElement).value).toBe("");
  });

  it("does not call onSend when input is only whitespace", () => {
    const onSend = vi.fn();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button"));

    expect(onSend).not.toHaveBeenCalled();
  });

  it("sends on Enter key (without Shift)", () => {
    const onSend = vi.fn();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "Hello" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: false });

    expect(onSend).toHaveBeenCalledWith("Hello");
  });

  it("does not send on Shift+Enter", () => {
    const onSend = vi.fn();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "Hello" } });
    fireEvent.keyDown(textarea, { key: "Enter", shiftKey: true });

    expect(onSend).not.toHaveBeenCalled();
  });

  it("uses translated placeholder text", () => {
    render(<ChatInput onSend={vi.fn()} />);
    // Our mock useTranslations returns "namespace.key" format
    expect(screen.getByPlaceholderText("chat.inputPlaceholder")).toBeInTheDocument();
  });
});
