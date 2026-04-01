import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import FeedbackModal from "../FeedbackModal";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("lucide-react-native", () => ({
  X: () => <span data-testid="x-icon" />,
  Check: () => <span data-testid="check-icon" />,
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      surfaceTertiary: "#f3f4f6",
      border: "#e5e7eb",
      textPrimary: "#111827",
      textSecondary: "#4b5563",
      textTertiary: "#5f6673",
      accent: "#e8654a",
      accentSoft: "#fef2f0",
      inputBg: "#ffffff",
      inputBorder: "#d1d5db",
      error: "#dc2626",
      success: "#16a34a",
      successBg: "#f0fdf4",
    },
  }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock("expo-router", () => ({
  usePathname: () => "/trips/abc/chat",
  useLocalSearchParams: () => ({ tripId: "abc" }),
}));

const mockSubmit = vi.fn();
const mockReset = vi.fn();
const mockFeedback = {
  submit: mockSubmit,
  isSubmitting: false,
  isSuccess: false,
  error: null as string | null,
  reset: mockReset,
};

vi.mock("@/lib/hooks/useFeedback", () => ({
  useFeedback: () => mockFeedback,
}));

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("FeedbackModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFeedback.isSubmitting = false;
    mockFeedback.isSuccess = false;
    mockFeedback.error = null;
  });

  it("renders when visible is true", () => {
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    expect(screen.getByText("feedback.title")).toBeInTheDocument();
  });

  it("renders feedback type chips", () => {
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    expect(screen.getByText("feedback.typeBug")).toBeInTheDocument();
    expect(screen.getByText("feedback.typeFeature")).toBeInTheDocument();
    expect(screen.getByText("feedback.typeGeneral")).toBeInTheDocument();
    expect(screen.getByText("feedback.typeChatQuality")).toBeInTheDocument();
  });

  it("renders message input with placeholder", () => {
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    expect(screen.getByPlaceholderText("feedback.messagePlaceholder")).toBeInTheDocument();
  });

  it("renders submit button", () => {
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    expect(screen.getByText("feedback.submit")).toBeInTheDocument();
  });

  it("submit button is disabled when message is too short", () => {
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText("feedback.messagePlaceholder");
    fireEvent.change(input, { target: { value: "short" } });
    // canSubmit requires >= 10 chars
    // The submit Pressable has disabled prop — check it doesn't call submit
    const submitBtn = screen.getByText("feedback.submit");
    fireEvent.click(submitBtn);
    expect(mockSubmit).not.toHaveBeenCalled();
  });

  it("calls submit with correct args when message is long enough", () => {
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    const input = screen.getByPlaceholderText("feedback.messagePlaceholder");
    fireEvent.change(input, { target: { value: "This is a long enough feedback message" } });

    const submitBtn = screen.getByText("feedback.submit");
    fireEvent.click(submitBtn);

    expect(mockSubmit).toHaveBeenCalledWith(
      "general",
      "This is a long enough feedback message",
      "/trips/abc/chat",
      "abc",
    );
  });

  it("calls onClose and reset when close button is pressed", () => {
    const onClose = vi.fn();
    render(<FeedbackModal visible={true} onClose={onClose} />);
    const xIcon = screen.getByTestId("x-icon");
    // The X icon is inside a Pressable — click its parent
    fireEvent.click(xIcon.parentElement!);
    expect(onClose).toHaveBeenCalled();
    expect(mockReset).toHaveBeenCalled();
  });

  it("shows success view when isSuccess is true", () => {
    mockFeedback.isSuccess = true;
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    expect(screen.getByText("feedback.success")).toBeInTheDocument();
    expect(screen.getByText("feedback.successDetail")).toBeInTheDocument();
    expect(screen.getByTestId("check-icon")).toBeInTheDocument();
  });

  it("shows error message when error is set", () => {
    mockFeedback.error = "Something went wrong";
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    expect(screen.getByText("feedback.error")).toBeInTheDocument();
  });

  it("allows selecting a different feedback type", () => {
    render(<FeedbackModal visible={true} onClose={vi.fn()} />);
    const bugChip = screen.getByText("feedback.typeBug");
    fireEvent.click(bugChip);

    // Now type a long enough message and submit
    const input = screen.getByPlaceholderText("feedback.messagePlaceholder");
    fireEvent.change(input, { target: { value: "This is a bug report message" } });

    const submitBtn = screen.getByText("feedback.submit");
    fireEvent.click(submitBtn);

    expect(mockSubmit).toHaveBeenCalledWith(
      "bug",
      "This is a bug report message",
      "/trips/abc/chat",
      "abc",
    );
  });
});
