import { useQuery } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AuthService } from "@gen/toqui/v1/auth_pb";
import { getConfig } from "@/lib/config";

/**
 * Fetch which auth providers the server has configured.
 * - `emailPassword` is always true (the OSS default).
 * - `googleOauth` is true only when the backend has GOOGLE_CLIENT_ID +
 *   GOOGLE_CLIENT_SECRET set. Self-hosted operators who don't configure
 *   Google should not see the Google sign-in button.
 *
 * The result is cached for the lifetime of the session — `staleTime: Infinity`
 * means React Query will never refetch it. The server-side flag is bound to
 * environment variables that can't change without a redeploy, so a single
 * fetch per page load is plenty.
 */
export function useAuthProviders() {
  return useQuery({
    queryKey: ["auth-providers"],
    queryFn: async () => {
      const transport = createConnectTransport({ baseUrl: getConfig().apiUrl });
      const client = createClient(AuthService, transport);
      return client.getAuthProviders({});
    },
    staleTime: Infinity,
  });
}
