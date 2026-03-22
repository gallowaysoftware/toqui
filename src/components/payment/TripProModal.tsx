"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Crown, X, Check, Sparkles, Mail, Users, Download } from "lucide-react";
import { useCreateCheckout, useValidatePayment } from "@/lib/hooks/usePayment";

// HelcimPay.js global functions (loaded via script tag)
declare global {
  function appendHelcimPayIframe(checkoutToken: string, allowExit?: boolean): void;
  function removeHelcimPayIframe(): void;
}

interface TripProModalProps {
  tripId: string;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const FEATURES = [
  { icon: Users, text: "Unlimited access to 800+ expert personas" },
  { icon: Sparkles, text: "Unlimited messages per day" },
  { icon: Mail, text: "Email forwarding for booking confirmations" },
  { icon: Download, text: "Export trip data and itineraries" },
];

export function TripProModal({ tripId, isOpen, onClose, onSuccess }: TripProModalProps) {
  const [step, setStep] = useState<"info" | "paying" | "success" | "error">("info");
  const [errorMessage, setErrorMessage] = useState("");
  const checkoutTokenRef = useRef<string>("");

  const createCheckout = useCreateCheckout();
  const validatePayment = useValidatePayment();

  // Load HelcimPay.js script
  useEffect(() => {
    if (!isOpen) return;

    const existing = document.getElementById("helcim-pay-js");
    if (existing) return;

    const script = document.createElement("script");
    script.id = "helcim-pay-js";
    script.src = "https://secure.helcim.app/helcim-pay/services/start.js";
    script.async = true;
    document.head.appendChild(script);
  }, [isOpen]);

  // Listen for HelcimPay.js messages
  const handleHelcimMessage = useCallback(
    (event: MessageEvent) => {
      const token = checkoutTokenRef.current;
      if (!token) return;

      const key = "helcim-pay-js-" + token;
      if (event.data?.eventName !== key) return;

      if (event.data.eventStatus === "SUCCESS") {
        const msg = event.data.eventMessage;
        validatePayment.mutate(
          {
            checkoutToken: token,
            responseData: msg.data,
            responseHash: msg.hash,
            tripId,
          },
          {
            onSuccess: () => {
              setStep("success");
              removeHelcimPayIframe();
              setTimeout(onSuccess, 1500);
            },
            onError: (err) => {
              setErrorMessage(err.message);
              setStep("error");
              removeHelcimPayIframe();
            },
          },
        );
      } else if (event.data.eventStatus === "ABORTED") {
        setErrorMessage("Payment was declined. Please try again.");
        setStep("error");
        removeHelcimPayIframe();
      } else if (event.data.eventStatus === "HIDE") {
        setStep("info");
      }
    },
    [tripId, validatePayment, onSuccess],
  );

  useEffect(() => {
    window.addEventListener("message", handleHelcimMessage);
    return () => window.removeEventListener("message", handleHelcimMessage);
  }, [handleHelcimMessage]);

  const handlePurchase = async () => {
    setErrorMessage("");
    try {
      const result = await createCheckout.mutateAsync({ tripId });
      checkoutTokenRef.current = result.checkout_token;
      setStep("paying");
      // Small delay to ensure script is loaded
      setTimeout(() => {
        appendHelcimPayIframe(result.checkout_token);
      }, 100);
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "Failed to initialize checkout");
      setStep("error");
    }
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      role="dialog"
      aria-modal="true"
      aria-label="Upgrade to Trip Pro"
    >
      <div className="bg-[var(--color-surface)] rounded-2xl border border-[var(--color-border)] shadow-xl max-w-md w-full overflow-hidden">
        {/* Header */}
        <div className="bg-gradient-to-r from-[var(--color-accent)] to-[var(--color-accent-hover)] p-6 text-white relative">
          <button
            onClick={onClose}
            className="absolute top-4 right-4 text-white/70 hover:text-white transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/50 rounded"
            aria-label="Close"
          >
            <X size={20} />
          </button>
          <div className="flex items-center gap-3 mb-2">
            <Crown size={28} />
            <h2 className="text-2xl font-bold">Trip Pro</h2>
          </div>
          <p className="text-white/80 text-sm">Unlock the full Toqui experience for this trip</p>
        </div>

        {/* Content */}
        <div className="p-6">
          {step === "info" && (
            <>
              <ul className="space-y-3 mb-6">
                {FEATURES.map(({ icon: Icon, text }) => (
                  <li key={text} className="flex items-center gap-3">
                    <div className="flex-shrink-0 w-8 h-8 rounded-lg bg-[var(--color-accent-soft)] flex items-center justify-center">
                      <Icon size={16} className="text-[var(--color-accent)]" />
                    </div>
                    <span className="text-sm text-[var(--color-text-secondary)]">{text}</span>
                  </li>
                ))}
              </ul>

              <div className="text-center mb-4">
                <span className="text-3xl font-bold text-[var(--color-text-primary)]">$12</span>
                <span className="text-sm text-[var(--color-text-tertiary)] ml-1">CAD / trip</span>
              </div>

              <button
                onClick={handlePurchase}
                disabled={createCheckout.isPending}
                className="w-full bg-[var(--color-accent)] text-white py-3 rounded-xl font-semibold hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {createCheckout.isPending ? (
                  <>
                    <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent" />
                    Setting up...
                  </>
                ) : (
                  <>
                    <Crown size={18} />
                    Upgrade Now
                  </>
                )}
              </button>

              <p className="text-xs text-[var(--color-text-tertiary)] text-center mt-3">
                One-time purchase. 24-hour refund policy.
                <br />
                Payments processed securely by Helcim.
              </p>
            </>
          )}

          {step === "paying" && (
            <div className="text-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)] mx-auto mb-4" />
              <p className="text-sm text-[var(--color-text-secondary)]">
                Complete your payment in the secure checkout window...
              </p>
            </div>
          )}

          {step === "success" && (
            <div className="text-center py-8">
              <div className="w-16 h-16 rounded-full bg-[var(--color-success-bg)] flex items-center justify-center mx-auto mb-4">
                <Check size={32} className="text-[var(--color-success)]" />
              </div>
              <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-1">
                Trip Pro Unlocked!
              </h3>
              <p className="text-sm text-[var(--color-text-secondary)]">
                You now have full access to all features for this trip.
              </p>
            </div>
          )}

          {step === "error" && (
            <div className="text-center py-6">
              <p className="text-sm text-[var(--color-error)] mb-4" role="alert">
                {errorMessage}
              </p>
              <button
                onClick={() => setStep("info")}
                className="text-sm text-[var(--color-accent)] hover:underline"
              >
                Try again
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
