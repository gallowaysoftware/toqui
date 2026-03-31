import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ChatInput } from "../ChatInput";

// Mock lucide icons to avoid SVG rendering issues in jsdom
vi.mock("lucide-react-native", () => ({
  Send: () => <span data-testid="send-icon" />,
  Paperclip: () => <span data-testid="paperclip-icon" />,
  X: () => <span data-testid="x-icon" />,
}));

describe("ChatInput", () => {
  describe("sending messages", () => {
    it("calls onSend with trimmed text when send button is pressed", () => {
      const onSend = vi.fn();
      render(<ChatInput onSend={onSend} />);

      const input = screen.getByPlaceholderText("Type a message...");
      fireEvent.change(input, { target: { value: "  Hello world  " } });

      const button = screen.getByRole("button", { name: "Send" });
      fireEvent.click(button);

      expect(onSend).toHaveBeenCalledTimes(1);
      expect(onSend).toHaveBeenCalledWith("Hello world", undefined);
    });

    it("clears input after sending", () => {
      const onSend = vi.fn();
      render(<ChatInput onSend={onSend} />);

      const input = screen.getByPlaceholderText("Type a message...") as HTMLInputElement;
      fireEvent.change(input, { target: { value: "Hello" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));

      expect(input.value).toBe("");
    });

    it("does not call onSend when input is empty", () => {
      const onSend = vi.fn();
      render(<ChatInput onSend={onSend} />);

      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      expect(onSend).not.toHaveBeenCalled();
    });

    it("does not call onSend when input contains only whitespace", () => {
      const onSend = vi.fn();
      render(<ChatInput onSend={onSend} />);

      const input = screen.getByPlaceholderText("Type a message...");
      fireEvent.change(input, { target: { value: "   \t\n  " } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));

      expect(onSend).not.toHaveBeenCalled();
    });
  });

  describe("disabled state", () => {
    it("does not call onSend when disabled even with valid text", () => {
      const onSend = vi.fn();
      render(<ChatInput onSend={onSend} disabled />);

      const input = screen.getByPlaceholderText("Type a message...");
      fireEvent.change(input, { target: { value: "Hello" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));

      expect(onSend).not.toHaveBeenCalled();
    });

    it("disables the send button when disabled prop is true", () => {
      render(<ChatInput onSend={vi.fn()} disabled />);
      const button = screen.getByRole("button", { name: "Send" });
      expect(button).toBeDisabled();
    });

    it("disables the send button when input is empty", () => {
      render(<ChatInput onSend={vi.fn()} />);
      const button = screen.getByRole("button", { name: "Send" });
      expect(button).toBeDisabled();
    });
  });

  describe("placeholder", () => {
    it("uses default placeholder text", () => {
      render(<ChatInput onSend={vi.fn()} />);
      expect(screen.getByPlaceholderText("Type a message...")).toBeInTheDocument();
    });

    it("uses custom placeholder when provided", () => {
      render(<ChatInput onSend={vi.fn()} placeholder="Ask anything..." />);
      expect(screen.getByPlaceholderText("Ask anything...")).toBeInTheDocument();
    });
  });

  describe("max length", () => {
    it("enforces maxLength of 10000 on the input", () => {
      render(<ChatInput onSend={vi.fn()} />);
      const input = screen.getByPlaceholderText("Type a message...") as HTMLInputElement;
      expect(input.maxLength).toBe(10000);
    });
  });
});
