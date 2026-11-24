import { useQuery } from "@tanstack/react-query";
import { listPayChannels, type PayChannelCapability } from "@/shared/api/payments-admin";

export type PayChannelOption = { value: string; label: string };

export function usePayChannels(): {
  channels: PayChannelCapability[];
  options: PayChannelOption[];
  labels: Record<string, string>;
  isLoading: boolean;
} {
  const query = useQuery({
    queryKey: ["pay-channels"],
    queryFn: listPayChannels,
    staleTime: 5 * 60_000,
  });

  const channels = query.data ?? [];
  const options = channels.map((c) => ({ value: c.channel, label: c.displayName }));
  const labels = Object.fromEntries(channels.map((c) => [c.channel, c.displayName]));

  return { channels, options, labels, isLoading: query.isLoading };
}
