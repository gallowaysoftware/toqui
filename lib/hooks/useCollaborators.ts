import { useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

export interface Collaborator {
  id: string;
  email: string;
  role: "owner" | "editor" | "viewer";
  invitedAt: string;
  acceptedAt: string | null;
  userId: string | null;
}

export interface AcceptInviteResult {
  trip: {
    id: string;
    title: string;
    description: string;
    destinationCountry: string;
    invitedBy: string;
  };
}

export function useCollaborators(tripId: string) {
  const { accessToken } = useAuth();

  const { data: collaborators = [], isLoading, error, isError, refetch } = useQuery<Collaborator[]>({
    queryKey: ["collaborators", tripId, accessToken],
    queryFn: async () => {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/trips/${tripId}/collaborators`,
        accessToken,
      );
      if (!res.ok) {
        throw new Error(`Failed to fetch collaborators: ${res.status}`);
      }
      const json = (await res.json()) as { collaborators: Collaborator[] };
      return json.collaborators;
    },
    enabled: !!accessToken && !!tripId,
  });

  return { collaborators, isLoading, error, isError, refetch };
}

export interface InviteResult {
  collaborator: Collaborator;
  /**
   * Whether the backend successfully delivered the invite email. When false,
   * `acceptUrl` is set so the inviter can share the link manually as a
   * fallback (e.g. Resend rejected the send because the sender domain isn't
   * verified, or no email service is configured).
   */
  emailSent: boolean;
  acceptUrl?: string;
}

export function useInviteCollaborator() {
  const { accessToken } = useAuth();
  const queryClient = useQueryClient();

  return useMutation<InviteResult, Error, { tripId: string; email: string; role: "editor" | "viewer" }>({
    mutationFn: async ({ tripId, email, role }) => {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/trips/${tripId}/invite`,
        accessToken,
        {
          method: "POST",
          body: JSON.stringify({ email, role }),
        },
      );
      if (!res.ok) {
        const body = await res.text().catch(() => "");
        throw new Error(body || `Failed to invite collaborator: ${res.status}`);
      }
      const json = (await res.json()) as {
        id: string;
        email: string;
        role: "owner" | "editor" | "viewer";
        invited_at: string;
        email_sent?: boolean;
        accept_url?: string;
      };
      return {
        collaborator: {
          id: json.id,
          email: json.email,
          role: json.role,
          invitedAt: json.invited_at,
          acceptedAt: null,
          userId: null,
        },
        emailSent: json.email_sent ?? true,
        acceptUrl: json.accept_url,
      };
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["collaborators", variables.tripId] });
    },
  });
}

export function useRemoveCollaborator() {
  const { accessToken } = useAuth();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ tripId, email }: { tripId: string; email: string }) => {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/trips/${tripId}/collaborators/${encodeURIComponent(email)}`,
        accessToken,
        { method: "DELETE" },
      );
      if (!res.ok) {
        throw new Error(`Failed to remove collaborator: ${res.status}`);
      }
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["collaborators", variables.tripId] });
    },
  });
}

export function useAcceptInvite() {
  const { accessToken } = useAuth();

  const acceptInvite = useCallback(
    async (token: string): Promise<AcceptInviteResult> => {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/trips/accept-invite`,
        accessToken,
        {
          method: "POST",
          body: JSON.stringify({ token }),
        },
      );
      if (!res.ok) {
        const body = await res.text().catch(() => "");
        if (res.status === 410) throw new Error("expired");
        if (res.status === 409) throw new Error("already_accepted");
        throw new Error(body || `Failed to accept invite: ${res.status}`);
      }
      return (await res.json()) as AcceptInviteResult;
    },
    [accessToken],
  );

  return { acceptInvite };
}
