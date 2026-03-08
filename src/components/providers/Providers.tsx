"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { GrpcProvider } from "./GrpcProvider";
import { AuthProvider } from "./AuthProvider";
import { ThemeProvider } from "./ThemeProvider";
import { AgeGate } from "@/components/auth/AgeGate";

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60 * 1000,
            retry: 1,
          },
        },
      }),
  );

  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <AgeGate>
            <GrpcProvider>
              {children}
            </GrpcProvider>
          </AgeGate>
        </AuthProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
