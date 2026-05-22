import { useMemo, useState } from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { useQuery } from '@tanstack/react-query';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Input, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { fetchAdminLogFilterOptions, type AdminLogFilterKind } from '@/features/adminLogs/api';
import {
  buildSubscribeFilters,
  fuzzyFilterLogEntries,
  isGlobalAdmin,
  redactEntryForCopy,
  type AdminLogEntry,
  type AdminLogFilterOption,
  type LogStatus
} from '@/features/adminLogs/model';
import { useAdminLogStream } from '@/features/adminLogs/useAdminLogStream';
import { useCurrentUser } from '@/features/profile/hooks';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { AppMenu } from '@/shared/ui/AppMenu';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { TopBar } from '@/shared/ui/TopBar';

const statusOptions: Array<{ label: string; value: LogStatus }> = [
  { label: 'OK', value: 'ok' },
  { label: 'Warn', value: 'warning' },
  { label: 'Error', value: 'error' }
];
const sourceOptions = ['', 'mqtt', 'mqtt-status', 'replay'];
const typeOptions = ['', 'quota', 'status', 'telemetry'];

export default function LogsScreen() {
  const { spec } = useAppTheme();
  const semantics = useThemeSemantics();
  const { authReady, authKey, token } = useAuthSession();
  const { allowed, waiting } = useRequireAuth();
  const currentUserQuery = useCurrentUser({ token, authKey, enabled: authReady && allowed });
  const isAdmin = isGlobalAdmin(currentUserQuery.data?.authorization.roles);
  const [selectedOptions, setSelectedOptions] = useState<AdminLogFilterOption[]>([]);
  const [statuses, setStatuses] = useState<LogStatus[]>([]);
  const [source, setSource] = useState('');
  const [typeCode, setTypeCode] = useState('');
  const [freetext, setFreetext] = useState('');
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const filters = useMemo(
    () => buildSubscribeFilters({ selectedOptions, statuses, source, typeCode }),
    [selectedOptions, source, statuses, typeCode]
  );
  const stream = useAdminLogStream({
    token,
    enabled: authReady && allowed && isAdmin,
    filters
  });
  const visibleEntries = useMemo(
    () => fuzzyFilterLogEntries(stream.entries, freetext),
    [freetext, stream.entries]
  );

  const addOption = (option: AdminLogFilterOption) => {
    setSelectedOptions((current) => {
      if (current.some((item) => item.kind === option.kind && item.id === option.id)) {
        return current;
      }
      return [...current, option];
    });
  };

  if (waiting || !allowed || currentUserQuery.isLoading) {
    return <BrandedLoadingState minHeight={260} message="Checking session..." />;
  }

  if (!isAdmin) {
    return (
      <YStack flex={1} backgroundColor="$background" testID="screen-logs-forbidden">
        <LogsTopBar />
        <YStack flex={1} alignItems="center" justifyContent="center" padding="$5" gap="$3">
          <MaterialCommunityIcons name="shield-lock-outline" size={40} color={semantics.statusWarning} />
          <Text fontSize="$7" fontWeight="800">Admin access required</Text>
          <Text maxWidth={520} textAlign="center" color="$colorMuted">
            Realtime MQTT logs are available only to users with the global admin role.
          </Text>
        </YStack>
      </YStack>
    );
  }

  return (
    <YStack flex={1} minHeight={0} backgroundColor="$background" testID="screen-logs">
      <LogsTopBar />
      <YStack flex={1} minHeight={0} padding="$3" gap="$3">
        <XStack
          alignItems="center"
          justifyContent="space-between"
          gap="$3"
          paddingVertical="$2"
          paddingHorizontal="$3"
          borderWidth={1}
          borderRadius={8}
          style={{ backgroundColor: semantics.sectionBackground, borderColor: semantics.sectionBorder }}
        >
          <XStack alignItems="center" gap="$2" flexWrap="wrap" flex={1}>
            <StatusDot state={stream.connectionState} />
            <Text fontSize="$3" fontWeight="800">
              {formatConnectionState(stream.connectionState)}
            </Text>
            <Text fontSize="$2" color="$colorMuted">
              replayed {stream.replayedCount} · visible {visibleEntries.length} · buffered {stream.entries.length}
            </Text>
            {stream.paused ? (
              <Text fontSize="$2" color="$colorMuted">
                pending {stream.pendingCount}
              </Text>
            ) : null}
          </XStack>
          <XStack gap="$2">
            <Button size="$3" chromeless onPress={() => stream.setPaused(!stream.paused)}>
              <MaterialCommunityIcons name={stream.paused ? 'play' : 'pause'} size={16} color={spec.colors.color} />
              <Text fontSize="$2" fontWeight="700">{stream.paused ? 'Resume' : 'Pause'}</Text>
            </Button>
            <Button size="$3" chromeless onPress={stream.clear}>
              <MaterialCommunityIcons name="broom" size={16} color={spec.colors.color} />
              <Text fontSize="$2" fontWeight="700">Clear</Text>
            </Button>
          </XStack>
        </XStack>

        <YStack gap="$2" padding="$3" borderWidth={1} borderRadius={8} style={{ borderColor: semantics.sectionBorder }}>
          <XStack gap="$2" flexWrap="wrap" alignItems="flex-start">
            <LogTypeahead kind="device" label="Device" token={token} onSelect={addOption} />
            <LogTypeahead kind="serial" label="Serial" token={token} onSelect={addOption} />
            <LogTypeahead kind="user" label="User email" token={token} onSelect={addOption} />
            <YStack minWidth={190} flex={1}>
              <Input
                size="$3"
                value={freetext}
                onChangeText={setFreetext}
                placeholder="Freetext fuzzy search"
                aria-label="Freetext fuzzy search"
              />
            </YStack>
          </XStack>

          <XStack gap="$2" flexWrap="wrap" alignItems="center">
            <SegmentLabel label="Status" />
            {statusOptions.map((option) => {
              const active = statuses.includes(option.value);
              return (
                <FilterButton
                  key={option.value}
                  active={active}
                  label={option.label}
                  onPress={() => setStatuses((current) => active ? current.filter((item) => item !== option.value) : [...current, option.value])}
                />
              );
            })}
            <SegmentLabel label="Source" />
            {sourceOptions.map((option) => (
              <FilterButton key={option || 'all-source'} active={source === option} label={option || 'All'} onPress={() => setSource(option)} />
            ))}
            <SegmentLabel label="Type" />
            {typeOptions.map((option) => (
              <FilterButton key={option || 'all-type'} active={typeCode === option} label={option || 'All'} onPress={() => setTypeCode(option)} />
            ))}
          </XStack>

          {selectedOptions.length > 0 ? (
            <XStack gap="$2" flexWrap="wrap">
              {selectedOptions.map((option) => (
                <Button
                  key={`${option.kind}:${option.id}`}
                  size="$2"
                  chromeless
                  onPress={() => setSelectedOptions((current) => current.filter((item) => item !== option))}
                >
                  <MaterialCommunityIcons name="close" size={14} color={spec.colors.colorMuted} />
                  <Text fontSize="$2" numberOfLines={1}>
                    {option.kind}: {option.label}
                  </Text>
                </Button>
              ))}
            </XStack>
          ) : null}
        </YStack>

        <YStack flex={1} minHeight={0} borderWidth={1} borderRadius={8} overflow="hidden" style={{ borderColor: semantics.sectionBorder }}>
          <LogTableHeader />
          <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingBottom: 24 }}>
            {visibleEntries.length === 0 ? (
              <YStack minHeight={180} alignItems="center" justifyContent="center" gap="$2">
                <MaterialCommunityIcons name="text-box-search-outline" size={30} color={spec.colors.colorMuted} />
                <Text fontSize="$4" fontWeight="700">Waiting for matching log entries</Text>
                <Text fontSize="$2" color="$colorMuted">Replay and live MQTT frames will appear here.</Text>
              </YStack>
            ) : (
              visibleEntries.map((entry) => (
                <LogRow
                  key={entry.id}
                  entry={entry}
                  expanded={expandedId === entry.id}
                  onToggle={() => setExpandedId((current) => current === entry.id ? null : entry.id)}
                />
              ))
            )}
          </ScrollView>
        </YStack>
      </YStack>
    </YStack>
  );
}

function LogsTopBar() {
  return (
    <TopBar
      eyebrow={(
        <BreadcrumbTrail
          items={[
            { label: 'Home', href: '/devices', icon: 'home-outline', hideLabel: true },
            { label: 'Logs', current: true }
          ]}
        />
      )}
      title={<BrandLogo compact={false} />}
      subtitle={<Text fontSize={11} color="$colorMuted">Realtime MQTT operations console</Text>}
      right={<AppMenu />}
    />
  );
}

function LogTypeahead({
  kind,
  label,
  token,
  onSelect
}: {
  kind: AdminLogFilterKind;
  label: string;
  token?: string;
  onSelect: (option: AdminLogFilterOption) => void;
}) {
  const semantics = useThemeSemantics();
  const [query, setQuery] = useState('');
  const trimmed = query.trim();
  const optionsQuery = useQuery({
    queryKey: ['admin-log-filter-options', kind, trimmed, token],
    queryFn: () => fetchAdminLogFilterOptions({ token, kind, query: trimmed, limit: 5 }),
    enabled: trimmed.length >= 2,
    staleTime: 30_000
  });

  return (
    <YStack minWidth={170} flex={1} gap={4}>
      <Input size="$3" value={query} onChangeText={setQuery} placeholder={label} aria-label={label} />
      {trimmed.length >= 2 && optionsQuery.data && optionsQuery.data.length > 0 ? (
        <YStack borderWidth={1} borderRadius={8} overflow="hidden" style={{ borderColor: semantics.sectionBorder }}>
          {optionsQuery.data.map((option) => (
            <Pressable
              key={`${option.kind}:${option.id}`}
              onPress={() => {
                onSelect(option);
                setQuery('');
              }}
              style={({ pressed }) => ({
                paddingHorizontal: 10,
                paddingVertical: 8,
                backgroundColor: pressed ? semantics.navItemHoverBackground : semantics.sectionBackground
              })}
            >
              <Text fontSize="$2" fontWeight="700" numberOfLines={1}>{option.label}</Text>
              <Text fontSize="$1" color="$colorMuted" numberOfLines={1}>{option.secondaryLabel}</Text>
            </Pressable>
          ))}
        </YStack>
      ) : null}
    </YStack>
  );
}

function LogTableHeader() {
  const semantics = useThemeSemantics();
  return (
    <XStack
      height={34}
      alignItems="center"
      paddingHorizontal="$3"
      borderBottomWidth={1}
      style={{ backgroundColor: semantics.sectionBackgroundStrong, borderBottomColor: semantics.sectionBorder }}
    >
      <Text width={96} fontSize="$1" fontWeight="800" color="$colorMuted">Severity</Text>
      <Text width={178} fontSize="$1" fontWeight="800" color="$colorMuted">Time</Text>
      <Text width={150} fontSize="$1" fontWeight="800" color="$colorMuted">Device</Text>
      <Text flex={1} fontSize="$1" fontWeight="800" color="$colorMuted">Summary</Text>
    </XStack>
  );
}

function LogRow({ entry, expanded, onToggle }: { entry: AdminLogEntry; expanded: boolean; onToggle: () => void }) {
  const { spec } = useAppTheme();
  const semantics = useThemeSemantics();
  const copied = JSON.stringify(redactEntryForCopy(entry), null, 2);
  return (
    <YStack borderBottomWidth={1} style={{ borderBottomColor: semantics.sectionBorder }}>
      <Pressable
        onPress={onToggle}
        style={({ pressed }) => ({
          minHeight: 42,
          justifyContent: 'center',
          backgroundColor: expanded ? semantics.navItemActiveBackground : pressed ? semantics.navItemHoverBackground : 'transparent'
        })}
      >
        <XStack alignItems="center" gap="$2" paddingHorizontal="$3" paddingVertical="$2">
          <XStack width={96} alignItems="center" gap="$2">
            <MaterialCommunityIcons name={expanded ? 'chevron-down' : 'chevron-right'} size={18} color={spec.colors.colorMuted} />
            <StatusBadge status={entry.status} />
          </XStack>
          <Text width={178} fontSize="$2" fontFamily="$body" numberOfLines={1}>{formatTime(entry.ts)}</Text>
          <Text width={150} fontSize="$2" numberOfLines={1}>{shortId(entry.deviceId)}</Text>
          <XStack flex={1} alignItems="center" gap="$2">
            <LogChip label={entry.source} />
            <LogChip label={entry.typeCode} />
            <Text flex={1} fontSize="$2" numberOfLines={1}>{entry.summary}</Text>
          </XStack>
        </XStack>
      </Pressable>
      {expanded ? (
        <YStack padding="$3" gap="$2" style={{ backgroundColor: semantics.sectionBackground }}>
          <XStack gap="$2" flexWrap="wrap">
            <Button size="$2" chromeless onPress={() => void copyText(copied)}>
              <MaterialCommunityIcons name="content-copy" size={14} color={spec.colors.color} />
              <Text fontSize="$2" fontWeight="700">Copy</Text>
            </Button>
            <LogChip label={`sourceKind: ${entry.sourceKind}`} />
            <LogChip label={`received: ${formatTime(entry.receivedTs)}`} />
          </XStack>
          <View style={{ maxHeight: 320 }}>
            <ScrollView horizontal>
              <Text fontSize={12} lineHeight={18} fontFamily="$body" selectable>
                {copied}
              </Text>
            </ScrollView>
          </View>
        </YStack>
      ) : null}
    </YStack>
  );
}

function FilterButton({ active, label, onPress }: { active: boolean; label: string; onPress: () => void }) {
  const semantics = useThemeSemantics();
  return (
    <Pressable
      onPress={onPress}
      style={({ pressed }) => ({
        minHeight: 34,
        paddingHorizontal: 12,
        borderRadius: 7,
        justifyContent: 'center',
        borderWidth: 1,
        borderColor: active ? semantics.navItemActiveBorder : semantics.navToggleBorder,
        backgroundColor: active ? semantics.navItemActiveBackground : pressed ? semantics.navItemHoverBackground : semantics.periodIdleBackground
      })}
    >
      <Text fontSize="$2" fontWeight="700" color={active ? '$color' : '$colorMuted'}>{label}</Text>
    </Pressable>
  );
}

function SegmentLabel({ label }: { label: string }) {
  return <Text fontSize="$2" fontWeight="800" color="$colorMuted" marginLeft="$1">{label}</Text>;
}

function StatusBadge({ status }: { status: LogStatus }) {
  const semantics = useThemeSemantics();
  const color = status === 'error' ? semantics.statusDanger : status === 'warning' ? semantics.statusWarning : semantics.statusSuccess;
  return (
    <XStack alignItems="center" gap={5}>
      <View style={{ width: 8, height: 8, borderRadius: 99, backgroundColor: color }} />
      <Text fontSize="$1" fontWeight="800" textTransform="uppercase">{status}</Text>
    </XStack>
  );
}

function StatusDot({ state }: { state: string }) {
  const semantics = useThemeSemantics();
  const color = state === 'live' ? semantics.statusSuccess : state === 'error' || state === 'forbidden' ? semantics.statusDanger : semantics.statusWarning;
  return <View style={{ width: 9, height: 9, borderRadius: 99, backgroundColor: color }} />;
}

function LogChip({ label }: { label: string }) {
  const semantics = useThemeSemantics();
  return (
    <YStack paddingHorizontal="$2" paddingVertical={3} borderRadius={7} style={{ backgroundColor: semantics.periodIdleBackground }}>
      <Text fontSize="$1" numberOfLines={1} color="$colorMuted">{label || 'unknown'}</Text>
    </YStack>
  );
}

function formatConnectionState(state: string): string {
  switch (state) {
    case 'replay':
      return 'Replaying recent logs';
    case 'live':
      return 'Live';
    case 'forbidden':
      return 'Forbidden';
    case 'error':
      return 'Stream error';
    case 'connecting':
      return 'Connecting';
    case 'closed':
      return 'Closed';
    default:
      return 'Idle';
  }
}

function formatTime(ts: number): string {
  if (!Number.isFinite(ts) || ts <= 0) {
    return '--';
  }
  return new Date(ts).toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

function shortId(value: string): string {
  return value.length <= 12 ? value : `${value.slice(0, 8)}...${value.slice(-4)}`;
}

async function copyText(value: string): Promise<void> {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    await navigator.clipboard.writeText(value);
  }
}
