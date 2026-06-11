import { createElement, useEffect, useMemo, useState, type ComponentProps, type ReactNode } from 'react';
import { Platform, Pressable, ScrollView, View } from 'react-native';
import { useIsFocused } from 'expo-router/react-navigation';
import { useLocalSearchParams, usePathname } from 'expo-router';
import { useQuery } from '@tanstack/react-query';
import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Button, Input, Text, XStack, YStack } from 'tamagui';
import { useAuthSession } from '@/features/auth/hooks';
import { useRequireAuth } from '@/features/auth/useRequireAuth';
import { fetchAdminLogFilterOptions, type AdminLogFilterKind } from '@/features/adminLogs/api';
import {
  ADMIN_LOG_TYPE_FILTER_OPTIONS,
  buildSubscribeFilters,
  DEFAULT_LOG_KEEP_LIMIT,
  fuzzyFilterLogEntries,
  isAdminLogsRouteActive,
  mergeAdminLogFilterOptions,
  redactEntryForCopy,
  resolveAdminLogsRouteState,
  toggleExclusiveStatusFilter,
  type AdminLogTypeFilterValue,
  type BufferedAdminLogEntry,
  type AdminLogFilterOption,
  type LogStatus
} from '@/features/adminLogs/model';
import { useAdminLogStream } from '@/features/adminLogs/useAdminLogStream';
import { CONNECTOR_CATALOG, formatProviderLabel } from '@/features/integrations/providerCatalog';
import { useCurrentUser } from '@/features/profile/hooks';
import { useAppTheme } from '@/shared/theme/useAppTheme';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { AppMenu } from '@/shared/ui/AppMenu';
import { BrandLogo } from '@/shared/ui/BrandLogo';
import { BrandedLoadingState } from '@/shared/ui/BrandedLoadingState';
import { BreadcrumbTrail } from '@/shared/ui/BreadcrumbTrail';
import { TopBar } from '@/shared/ui/TopBar';
import {
  PULSE_PAGE_SECTION_GAP,
  resolvePageHorizontalPaddingPx,
  useNavigationShellMetrics
} from '@/shared/ui/navigationShell';
import { canAccessPulseLogs, isPulseGlobalAdmin } from '@/shared/authz/pulseRoles';

const statusOptions: Array<{ label: string; value: LogStatus }> = [
  { label: 'OK', value: 'ok' },
  { label: 'Warn', value: 'warning' },
  { label: 'Error', value: 'error' }
];
const providerOptions = [
  { value: '', label: 'All providers' },
  ...CONNECTOR_CATALOG.map((provider) => ({ value: provider.id, label: provider.title }))
];
const keepOptions = [25, 50, 100, 200].map((value) => ({ value: String(value), label: `Keep ${value}` }));
const logTableColumns = {
  severity: 96,
  time: 178,
  device: 190,
  user: 140
};
type MaterialIconName = ComponentProps<typeof MaterialCommunityIcons>['name'];

export default function LogsScreen() {
  const { spec } = useAppTheme();
  const semantics = useThemeSemantics();
  const { contentWidth } = useNavigationShellMetrics();
  const pagePadding = resolvePageHorizontalPaddingPx(contentWidth);
  const { authReady, authKey, token } = useAuthSession();
  const { allowed, waiting } = useRequireAuth();
  const currentUserQuery = useCurrentUser({ token, authKey, enabled: authReady && allowed });
  const isFocused = useIsFocused();
  const pathname = usePathname();
  const logsRouteActive = isAdminLogsRouteActive({
    isFocused,
    pathname,
    platformOS: Platform.OS
  });
  const routeParams = useLocalSearchParams<{ device?: string | string[]; deviceId?: string | string[] }>();
  const routeDeviceParam = routeParams.device;
  const routeDeviceIdParam = routeParams.deviceId;
  const routeDeviceId = useMemo(
    () => resolveAdminLogsRouteState({ device: routeDeviceParam, deviceId: routeDeviceIdParam }).deviceId,
    [routeDeviceParam, routeDeviceIdParam]
  );
  const roles = currentUserQuery.data?.authorization.roles;
  const deviceCount = currentUserQuery.data?.authorization.deviceCount;
  const isAdmin = isPulseGlobalAdmin(roles);
  const canReadLogs = canAccessPulseLogs({ roles, deviceCount });
  const [selectedOptions, setSelectedOptions] = useState<AdminLogFilterOption[]>([]);
  const [statuses, setStatuses] = useState<LogStatus[]>([]);
  const [provider, setProvider] = useState('');
  const [typeFilter, setTypeFilter] = useState<AdminLogTypeFilterValue>('');
  const [freetext, setFreetext] = useState('');
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const [maxEntries, setMaxEntries] = useState(DEFAULT_LOG_KEEP_LIMIT);
  const selectedTypeFilter = useMemo(
    () => ADMIN_LOG_TYPE_FILTER_OPTIONS.find((option) => option.value === typeFilter),
    [typeFilter]
  );
  const filters = useMemo(
    () =>
      buildSubscribeFilters({
        selectedOptions,
        deviceIds: routeDeviceId ? [routeDeviceId] : [],
        statuses,
        provider,
        typeCodes: selectedTypeFilter?.typeCodes ?? [],
        typeCodeSuffixes: selectedTypeFilter?.typeCodeSuffixes ?? []
      }),
    [selectedOptions, provider, routeDeviceId, selectedTypeFilter, statuses]
  );
  const filtersKey = useMemo(() => JSON.stringify(filters), [filters]);
  const stream = useAdminLogStream({
    token,
    enabled: authReady && allowed && canReadLogs,
    active: logsRouteActive,
    filters,
    maxEntries,
    holdVisible: expandedKey !== null
  });
  const visibleEntries = useMemo(
    () => fuzzyFilterLogEntries(stream.entries, freetext),
    [freetext, stream.entries]
  );
  const visibleDeviceIds = useMemo(() => uniqueVisibleDeviceIds(visibleEntries), [visibleEntries]);
  const metadataDeviceIds = useMemo(
    () => uniqueStrings([...(routeDeviceId ? [routeDeviceId] : []), ...visibleDeviceIds]),
    [routeDeviceId, visibleDeviceIds]
  );
  const visibleDeviceIdsKey = useMemo(() => visibleDeviceIds.join('|'), [visibleDeviceIds]);
  const metadataDeviceIdsKey = useMemo(() => metadataDeviceIds.join('|'), [metadataDeviceIds]);
  const deviceMetadataQuery = useQuery({
    queryKey: ['admin-log-device-metadata', metadataDeviceIdsKey, authKey],
    queryFn: () => fetchMetadataForDeviceIds({ token, kind: 'device', deviceIds: metadataDeviceIds }),
    enabled: logsRouteActive && authReady && allowed && canReadLogs && metadataDeviceIds.length > 0,
    staleTime: 60_000
  });
  const userMetadataQuery = useQuery({
    queryKey: ['admin-log-user-metadata', visibleDeviceIdsKey, authKey],
    queryFn: () => fetchMetadataForDeviceIds({ token, kind: 'user', deviceIds: visibleDeviceIds }),
    enabled: logsRouteActive && isAdmin && authReady && allowed && canReadLogs && visibleDeviceIds.length > 0,
    staleTime: 60_000
  });
  const userLabelByDeviceId = useMemo(
    () => buildUserLabelByDeviceId([...(userMetadataQuery.data ?? []), ...selectedOptions]),
    [selectedOptions, userMetadataQuery.data]
  );
  const deviceLabelByDeviceId = useMemo(
    () => buildDeviceLabelByDeviceId([...(deviceMetadataQuery.data ?? []), ...selectedOptions]),
    [deviceMetadataQuery.data, selectedOptions]
  );

  useEffect(() => {
    setExpandedKey(null);
  }, [filtersKey, freetext, maxEntries]);

  const addOption = (option: AdminLogFilterOption) => {
    setSelectedOptions((current) => mergeAdminLogFilterOptions([...current, option]));
  };
  const updateProvider = (value: string) => {
    setProvider(value);
    setSelectedOptions((current) => current.filter((option) => option.kind === 'user'));
  };

  const clearLogs = () => {
    setExpandedKey(null);
    stream.clear();
  };

  if (waiting || !allowed || currentUserQuery.isLoading) {
    return <BrandedLoadingState minHeight={260} message="Checking session..." />;
  }

  if (!canReadLogs) {
    return (
      <YStack flex={1} backgroundColor="$background" testID="screen-logs-forbidden">
        <LogsTopBar />
        <YStack flex={1} alignItems="center" justifyContent="center" padding="$5" gap="$3">
          <MaterialCommunityIcons name="shield-lock-outline" size={40} color={semantics.statusWarning} />
          <Text fontSize="$7" fontWeight="800">Logs unavailable</Text>
          <Text maxWidth={520} textAlign="center" color="$colorMuted">
            Realtime MQTT logs are available to device owners and global admins.
          </Text>
        </YStack>
      </YStack>
    );
  }

  return (
    <YStack flex={1} minHeight={0} backgroundColor="$background" testID="screen-logs">
      <LogsTopBar />
      <YStack
        flex={1}
        minHeight={0}
        paddingHorizontal={pagePadding}
        paddingVertical={PULSE_PAGE_SECTION_GAP}
        gap={PULSE_PAGE_SECTION_GAP}
      >
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
            <Text fontSize="$3" fontWeight="800" testID="logs-connection-state">
              {formatConnectionState(stream.connectionState)}
            </Text>
            <Text fontSize="$2" color="$colorMuted" testID="logs-stream-counts">
              replayed {stream.replayedCount} · visible {visibleEntries.length} · buffered {stream.entries.length}
            </Text>
            {stream.paused ? (
              <Text fontSize="$2" color="$colorMuted">
                pending {stream.pendingCount}
              </Text>
            ) : null}
          </XStack>
          <XStack gap="$2" alignItems="center">
            <LogCompactSelect
              label="Rows to keep"
              value={String(maxEntries)}
              options={keepOptions}
              onValueChange={(value) => setMaxEntries(Number(value))}
              testID="logs-keep-limit"
            />
            <Button size="$3" chromeless onPress={() => stream.setPaused(!stream.paused)}>
              <MaterialCommunityIcons name={stream.paused ? 'play' : 'pause'} size={16} color={spec.colors.color} />
              <Text fontSize="$2" fontWeight="700">{stream.paused ? 'Resume' : 'Pause'}</Text>
            </Button>
            <Button size="$3" chromeless onPress={clearLogs}>
              <MaterialCommunityIcons name="broom" size={16} color={spec.colors.color} />
              <Text fontSize="$2" fontWeight="700">Clear</Text>
            </Button>
          </XStack>
        </XStack>

        <YStack
          gap="$3"
          padding="$3"
          borderWidth={1}
          borderRadius={8}
          style={{
            backgroundColor: semantics.sectionBackground,
            borderColor: semantics.sectionBorder,
            position: 'relative',
            zIndex: 5
          }}
        >
          <XStack gap="$3" flexWrap="wrap" alignItems="flex-start">
            <LogSelectField
              label="Provider"
              icon="cloud-outline"
              value={provider}
              options={providerOptions}
              onValueChange={updateProvider}
              testID="logs-provider-select"
              minWidth={190}
              grow={0.75}
            />
            <LogTypeahead kind="device" label="Device" provider={provider} token={token} authKey={authKey} onSelect={addOption} />
            {isAdmin ? (
              <LogTypeahead kind="user" label="Email" token={token} authKey={authKey} onSelect={addOption} />
            ) : null}
            <LogTypeahead kind="serial" label="Serial" provider={provider} token={token} authKey={authKey} onSelect={addOption} />
            <LogFilterField label="Freetext" icon="text-search" minWidth={280} grow={1.45}>
              <LogFilterInput
                value={freetext}
                onChangeText={setFreetext}
                placeholder="Search all visible rows…"
                aria-label="Freetext fuzzy search"
              />
            </LogFilterField>
          </XStack>

          <XStack gap="$3" flexWrap="wrap" alignItems="flex-start">
            <FilterSegment label="Status">
              {statusOptions.map((option) => {
                const active = statuses.includes(option.value);
                return (
                  <FilterButton
                    key={option.value}
                    active={active}
                    label={option.label}
                    onPress={() => setStatuses((current) => toggleExclusiveStatusFilter(current, option.value))}
                  />
                );
              })}
            </FilterSegment>
            <FilterSegment label="Type">
              {ADMIN_LOG_TYPE_FILTER_OPTIONS.map((option) => (
                <FilterButton
                  key={option.value || 'all-type'}
                  active={typeFilter === option.value}
                  label={option.label}
                  onPress={() => setTypeFilter((current) => (option.value === '' || current !== option.value ? option.value : ''))}
                />
              ))}
            </FilterSegment>
          </XStack>

          {routeDeviceId || selectedOptions.length > 0 ? (
            <XStack gap="$2" flexWrap="wrap">
              {routeDeviceId ? (
                <LogFilterChip
                  icon="link-variant"
                  label={`device: ${deviceLabelByDeviceId.get(routeDeviceId) ?? shortId(routeDeviceId)}`}
                />
              ) : null}
              {selectedOptions.map((option) => (
                <Button
                  key={`${option.kind}:${option.id}`}
                  size="$2"
                  chromeless
                  onPress={() => setSelectedOptions((current) => current.filter((item) => item !== option))}
                >
                  <MaterialCommunityIcons name="close" size={14} color={spec.colors.colorMuted} />
                  <Text fontSize="$2" numberOfLines={1}>
                    {formatOptionKind(option.kind)}: {option.label}
                  </Text>
                </Button>
              ))}
            </XStack>
          ) : null}
        </YStack>

        <YStack
          flex={1}
          minHeight={0}
          borderWidth={1}
          borderRadius={8}
          overflow="hidden"
          testID="logs-table"
          style={{ borderColor: semantics.sectionBorder, position: 'relative', zIndex: 1 }}
        >
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
                  key={entry.rowKey}
                  entry={entry}
                  expanded={expandedKey === entry.rowKey}
                  isAdmin={isAdmin}
                  deviceLabelByDeviceId={deviceLabelByDeviceId}
                  userLabelByDeviceId={userLabelByDeviceId}
                  onToggle={() => setExpandedKey((current) => current === entry.rowKey ? null : entry.rowKey)}
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
  provider,
  token,
  authKey,
  onSelect
}: {
  kind: AdminLogFilterKind;
  label: string;
  provider?: string;
  token?: string;
  authKey: string;
  onSelect: (option: AdminLogFilterOption) => void;
}) {
  const semantics = useThemeSemantics();
  const [query, setQuery] = useState('');
  const trimmed = query.trim();
  const lookupProvider = kind === 'device' || kind === 'serial' ? provider : undefined;
  const optionsQuery = useQuery({
    queryKey: ['admin-log-filter-options', kind, trimmed, lookupProvider ?? '', authKey],
    queryFn: () => fetchAdminLogFilterOptions({ token, kind, query: trimmed, limit: 5, provider: lookupProvider }),
    enabled: trimmed.length >= 2,
    staleTime: 30_000
  });

  return (
    <LogFilterField label={label} icon={filterIconForKind(kind)}>
      <LogFilterInput
        value={query}
        onChangeText={setQuery}
        placeholder={`Search ${label.toLowerCase()}…`}
        aria-label={label}
      />
      {trimmed.length >= 2 && optionsQuery.data && optionsQuery.data.length > 0 ? (
        <YStack
          testID={`logs-typeahead-menu-${kind}`}
          borderWidth={1}
          borderRadius={8}
          overflow="hidden"
          style={{
            backgroundColor: semantics.sectionBackgroundStrong,
            borderColor: semantics.sectionBorder,
            boxShadow: '0 18px 46px rgba(0, 0, 0, 0.32)',
            left: 0,
            maxHeight: 240,
            position: 'absolute',
            right: 0,
            top: 66,
            zIndex: 50
          }}
        >
          {optionsQuery.data.map((option) => (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={`Use ${option.label}`}
              key={`${option.kind}:${option.id}`}
              onPress={() => {
                onSelect(option);
                setQuery('');
              }}
              style={({ pressed }) => ({
                paddingHorizontal: 10,
                paddingVertical: 8,
                backgroundColor: pressed ? semantics.navItemHoverBackground : semantics.sectionBackgroundStrong
              })}
            >
              <Text fontSize="$2" fontWeight="700" numberOfLines={1}>{option.label}</Text>
              <Text fontSize="$1" color="$colorMuted" numberOfLines={1}>{option.secondaryLabel}</Text>
            </Pressable>
          ))}
        </YStack>
      ) : null}
    </LogFilterField>
  );
}

type LogSelectOption = {
  value: string;
  label: string;
};

function LogSelectField({
  label,
  icon,
  value,
  options,
  onValueChange,
  testID,
  minWidth,
  grow
}: {
  label: string;
  icon: MaterialIconName;
  value: string;
  options: LogSelectOption[];
  onValueChange: (value: string) => void;
  testID: string;
  minWidth?: number;
  grow?: number;
}) {
  return (
    <LogFilterField label={label} icon={icon} minWidth={minWidth} grow={grow}>
      <LogSelectControl
        label={label}
        value={value}
        options={options}
        onValueChange={onValueChange}
        testID={testID}
      />
    </LogFilterField>
  );
}

function LogCompactSelect({
  label,
  value,
  options,
  onValueChange,
  testID
}: {
  label: string;
  value: string;
  options: LogSelectOption[];
  onValueChange: (value: string) => void;
  testID: string;
}) {
  return (
    <LogSelectControl
      compact
      label={label}
      value={value}
      options={options}
      onValueChange={onValueChange}
      testID={testID}
    />
  );
}

function LogSelectControl({
  compact = false,
  label,
  value,
  options,
  onValueChange,
  testID
}: {
  compact?: boolean;
  label: string;
  value: string;
  options: LogSelectOption[];
  onValueChange: (value: string) => void;
  testID: string;
}) {
  const semantics = useThemeSemantics();
  const { spec } = useAppTheme();
  const selectedLabel = options.find((option) => option.value === value)?.label ?? value;

  if (Platform.OS === 'web') {
    return createElement(
      'select',
      {
        value,
        'aria-label': label,
        'data-testid': testID,
        onChange: (event: { target: { value: string } }) => onValueChange(event.target.value),
        style: {
          width: compact ? undefined : '100%',
          minWidth: compact ? 104 : undefined,
          minHeight: compact ? 34 : 42,
          borderRadius: 8,
          borderWidth: 1,
          borderStyle: 'solid',
          borderColor: semantics.periodIdleBorder,
          backgroundColor: semantics.periodIdleBackground,
          color: spec.colors.color,
          padding: compact ? '0 28px 0 10px' : '0 34px 0 12px',
          fontSize: compact ? 13 : 14,
          fontWeight: 800
        }
      },
      options.map((option) =>
        createElement(
          'option',
          {
            key: option.value,
            value: option.value
          },
          option.label
        )
      )
    );
  }

  return (
    <Button
      size={compact ? '$2' : '$3'}
      minHeight={compact ? 34 : 42}
      borderWidth={1}
      borderRadius={8}
      testID={testID}
      style={{
        backgroundColor: semantics.periodIdleBackground,
        borderColor: semantics.periodIdleBorder
      }}
      onPress={() => {
        const index = options.findIndex((option) => option.value === value);
        const next = options[(index + 1) % options.length];
        if (next) {
          onValueChange(next.value);
        }
      }}
    >
      <Text fontSize={compact ? '$1' : '$2'} fontWeight="800" numberOfLines={1}>
        {selectedLabel}
      </Text>
      <MaterialCommunityIcons name="chevron-down" size={14} color={semantics.subtleText} />
    </Button>
  );
}

function LogFilterField({
  label,
  icon,
  children,
  minWidth = 220,
  grow = 1
}: {
  label: string;
  icon: MaterialIconName;
  children: ReactNode;
  minWidth?: number;
  grow?: number;
}) {
  const { spec } = useAppTheme();
  return (
    <YStack gap={6} style={{ minWidth, flexBasis: 0, flexGrow: grow, overflow: 'visible', position: 'relative' }}>
      <XStack alignItems="center" gap={6} paddingHorizontal={2} minHeight={18}>
        <MaterialCommunityIcons name={icon} size={14} color={spec.colors.colorMuted} />
        <Text fontSize="$1" fontWeight="800" color="$colorMuted" textTransform="uppercase">
          {label}
        </Text>
      </XStack>
      {children}
    </YStack>
  );
}

function LogFilterInput(props: ComponentProps<typeof Input>) {
  const semantics = useThemeSemantics();
  return (
    <Input
      size="$3"
      minHeight={42}
      borderWidth={1}
      borderRadius={8}
      paddingHorizontal={12}
      fontSize="$3"
      fontWeight="700"
      placeholderTextColor={semantics.subtleText}
      focusStyle={{ borderColor: '$accentColor', backgroundColor: '$backgroundHover' }}
      style={{
        backgroundColor: semantics.periodIdleBackground,
        borderColor: semantics.periodIdleBorder
      }}
      {...props}
    />
  );
}

function filterIconForKind(kind: AdminLogFilterKind): MaterialIconName {
  switch (kind) {
    case 'serial':
      return 'barcode-scan';
    case 'user':
      return 'account-search-outline';
    default:
      return 'devices';
  }
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
      <Text width={logTableColumns.severity} fontSize="$1" fontWeight="800" color="$colorMuted">Severity</Text>
      <Text width={logTableColumns.time} fontSize="$1" fontWeight="800" color="$colorMuted">Time</Text>
      <Text width={logTableColumns.device} fontSize="$1" fontWeight="800" color="$colorMuted">Device</Text>
      <Text width={logTableColumns.user} fontSize="$1" fontWeight="800" color="$colorMuted">User</Text>
      <Text flex={1} fontSize="$1" fontWeight="800" color="$colorMuted">Summary</Text>
    </XStack>
  );
}

function LogRow({
  entry,
  expanded,
  isAdmin,
  deviceLabelByDeviceId,
  userLabelByDeviceId,
  onToggle
}: {
  entry: BufferedAdminLogEntry;
  expanded: boolean;
  isAdmin: boolean;
  deviceLabelByDeviceId: ReadonlyMap<string, string>;
  userLabelByDeviceId: ReadonlyMap<string, string>;
  onToggle: () => void;
}) {
  const { spec } = useAppTheme();
  const semantics = useThemeSemantics();
  const copied = useMemo(() => (expanded ? JSON.stringify(redactEntryForCopy(entry), null, 2) : ''), [entry, expanded]);
  const deviceLabel = displayDeviceLabel(entry, deviceLabelByDeviceId);
  const userLabel = displayUserLabel(entry, userLabelByDeviceId, isAdmin);
  const providerLabel = displayProvider(entry);
  return (
    <YStack borderBottomWidth={1} style={{ borderBottomColor: semantics.sectionBorder }}>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={`${expanded ? 'Collapse' : 'Expand'} log entry ${entry.summary}`}
        onPress={onToggle}
        style={({ pressed }) => ({
          minHeight: 42,
          justifyContent: 'center',
          backgroundColor: expanded ? semantics.navItemActiveBackground : pressed ? semantics.navItemHoverBackground : 'transparent'
        })}
      >
        <XStack alignItems="center" gap="$2" paddingHorizontal="$3" paddingVertical="$2">
          <XStack width={logTableColumns.severity} alignItems="center" gap="$2">
            <MaterialCommunityIcons name={expanded ? 'chevron-down' : 'chevron-right'} size={18} color={spec.colors.colorMuted} />
            <StatusBadge status={entry.status} />
          </XStack>
          <Text width={logTableColumns.time} fontSize="$2" fontFamily="$body" numberOfLines={1}>{formatTime(entry.ts)}</Text>
          <YStack width={logTableColumns.device} gap={1}>
            <Text fontSize="$2" fontWeight="700" numberOfLines={1}>{deviceLabel.primary}</Text>
            {deviceLabel.secondary ? (
              <Text fontSize="$1" color="$colorMuted" numberOfLines={1}>{deviceLabel.secondary}</Text>
            ) : null}
          </YStack>
          <Text width={logTableColumns.user} fontSize="$2" color="$colorMuted" numberOfLines={1}>{userLabel}</Text>
          <XStack flex={1} alignItems="center" gap="$2">
            <LogChip label={providerLabel} />
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
          <View style={{ maxHeight: 360, overflow: 'hidden' }}>
            <ScrollView style={{ maxHeight: 360 }} contentContainerStyle={{ paddingBottom: 8 }}>
              <ScrollView horizontal>
                <Text
                  fontSize={12}
                  lineHeight={18}
                  selectable
                  style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' }}
                >
                  {copied}
                </Text>
              </ScrollView>
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
      accessibilityRole="button"
      accessibilityLabel={`${active ? 'Disable' : 'Enable'} ${label} filter`}
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

function FilterSegment({ label, children }: { label: string; children: ReactNode }) {
  return (
    <YStack gap="$2" style={{ minWidth: 210, flexGrow: 1 }}>
      <Text fontSize="$1" fontWeight="800" color="$colorMuted" textTransform="uppercase" marginLeft="$1">
        {label}
      </Text>
      <XStack gap="$2" flexWrap="wrap">
        {children}
      </XStack>
    </YStack>
  );
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

function LogFilterChip({ icon, label }: { icon: MaterialIconName; label: string }) {
  const { spec } = useAppTheme();
  return (
    <Button size="$2" chromeless disabled>
      <MaterialCommunityIcons name={icon} size={14} color={spec.colors.colorMuted} />
      <Text fontSize="$2" numberOfLines={1}>{label}</Text>
    </Button>
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

function displayProvider(entry: BufferedAdminLogEntry): string {
  const provider = labelValue(entry, ['provider', 'Provider']);
  return provider ? formatProviderLabel(provider) : 'unknown';
}

function displayDeviceLabel(
  entry: BufferedAdminLogEntry,
  deviceLabelByDeviceId: ReadonlyMap<string, string>
): { primary: string; secondary: string } {
  const name =
    deviceLabelByDeviceId.get(entry.deviceId) ??
    labelValue(entry, ['deviceName', 'device_name', 'productName', 'product_name']);
  const shortDeviceId = shortId(entry.deviceId);
  if (!name) {
    return { primary: shortDeviceId, secondary: '' };
  }
  return { primary: `${name} <${shortDeviceId}>`, secondary: '' };
}

function displayUserLabel(
  entry: BufferedAdminLogEntry,
  userLabelByDeviceId: ReadonlyMap<string, string>,
  isAdmin: boolean
): string {
  const metadataLabel = userLabelByDeviceId.get(entry.deviceId);
  if (metadataLabel) {
    return metadataLabel;
  }
  const streamLabel = labelValue(entry, ['userEmail', 'user_email', 'ownerEmail', 'owner_email', 'email']);
  if (streamLabel) {
    return maskEmailForLogs(streamLabel);
  }
  return isAdmin ? '--' : 'You';
}

function labelValue(entry: BufferedAdminLogEntry, keys: string[]): string {
  for (const key of keys) {
    const value = entry.labels[key]?.trim();
    if (value) {
      return value;
    }
  }
  return '';
}

function formatOptionKind(kind: AdminLogFilterKind): string {
  switch (kind) {
    case 'user':
      return 'email';
    case 'serial':
      return 'serial';
    default:
      return 'device';
  }
}

function shortId(value: string): string {
  return value.length <= 12 ? value : `${value.slice(0, 8)}...${value.slice(-4)}`;
}

function buildUserLabelByDeviceId(options: AdminLogFilterOption[]): ReadonlyMap<string, string> {
  const labels = new Map<string, string>();
  for (const option of options) {
    if (option.kind !== 'user') {
      continue;
    }
    const label = maskEmailForLogs(option.label);
    for (const deviceId of option.deviceIds) {
      if (!labels.has(deviceId)) {
        labels.set(deviceId, label);
      }
    }
  }
  return labels;
}

function buildDeviceLabelByDeviceId(options: AdminLogFilterOption[]): ReadonlyMap<string, string> {
  const labels = new Map<string, string>();
  for (const option of options) {
    if (option.kind !== 'device') {
      continue;
    }
    const label = option.label.trim();
    if (!label) {
      continue;
    }
    for (const deviceId of option.deviceIds) {
      if (!labels.has(deviceId)) {
        labels.set(deviceId, label);
      }
    }
  }
  return labels;
}

async function fetchMetadataForDeviceIds(input: {
  token?: string;
  kind: 'device' | 'user';
  deviceIds: string[];
}): Promise<AdminLogFilterOption[]> {
  const chunks = chunkValues(input.deviceIds, 50);
  const pages = await Promise.all(
    chunks.map((deviceIds) =>
      fetchAdminLogFilterOptions({
        token: input.token,
        kind: input.kind,
        query: '',
        limit: 50,
        deviceIds
      })
    )
  );
  return mergeAdminLogFilterOptions(pages.flat());
}

function uniqueVisibleDeviceIds(entries: BufferedAdminLogEntry[]): string[] {
  const ids = new Set<string>();
  for (const entry of entries) {
    const deviceId = entry.deviceId.trim();
    if (deviceId) {
      ids.add(deviceId);
    }
  }
  return [...ids].sort();
}

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))].sort();
}

function chunkValues<T>(values: T[], size: number): T[][] {
  const chunks: T[][] = [];
  for (let index = 0; index < values.length; index += size) {
    chunks.push(values.slice(index, index + size));
  }
  return chunks;
}

function maskEmailForLogs(value: string): string {
  const email = value.trim();
  const atIndex = email.indexOf('@');
  if (atIndex <= 0) {
    return email === '<redacted>' ? email : '<redacted>';
  }
  const local = email.slice(0, atIndex);
  const domain = email.slice(atIndex + 1).split('.')[0] ?? '';
  return `${maskEmailPart(local)}***@${maskEmailPart(domain)}***`;
}

function maskEmailPart(value: string): string {
  const trimmed = value.trim();
  if (trimmed.length <= 2) {
    return trimmed.slice(0, 1);
  }
  return trimmed.slice(0, 2);
}

async function copyText(value: string): Promise<void> {
  if (typeof navigator !== 'undefined' && navigator.clipboard) {
    await navigator.clipboard.writeText(value);
  }
}
