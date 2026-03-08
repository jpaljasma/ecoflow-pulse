import { useLocalSearchParams, useRouter } from 'expo-router';
import { Platform, ScrollView } from 'react-native';
import * as Linking from 'expo-linking';
import { Button, Text, XStack, YStack } from 'tamagui';
import { TopBar } from '@/shared/ui/TopBar';
import { Card } from '@/shared/ui/Card';
import { AppMenu } from '@/shared/ui/AppMenu';
import {
  AVOIDED_EMISSIONS_FACTORS,
  AVOIDED_EMISSIONS_FACTOR_VERSION,
  ENERGY_IMPACT_REFERENCE_DOC_URL,
  GENERIC_TREE_CO2_REMOVED_KG_PER_YEAR,
  GENERIC_TREE_KWH_PER_YEAR,
  PV_LIFECYCLE_CO2E_KG_PER_KWH,
  TREE_EQUIVALENT_FACTOR_VERSION,
  TREE_EQUIVALENT_REFERENCE_DOC_URL
} from '@/features/energy-impact/model';

function openReferenceDoc(url: string): void {
  if (Platform.OS === 'web' && typeof window !== 'undefined') {
    window.open(url, '_blank', 'noopener,noreferrer');
    return;
  }
  void Linking.openURL(url);
}

function highlightBorder(focus: string | undefined, key: string): string {
  return focus === key ? 'rgba(255,159,10,0.55)' : 'rgba(120,120,128,0.24)';
}

export default function EnergyImpactScreen() {
  const router = useRouter();
  const { focus } = useLocalSearchParams<{ focus?: string }>();
  const nyup = AVOIDED_EMISSIONS_FACTORS.NYUP;
  const usAverage = AVOIDED_EMISSIONS_FACTORS.US_AVG;

  return (
    <YStack flex={1} backgroundColor="$background" paddingHorizontal="$4" paddingVertical="$4" gap="$4">
      <TopBar
        left={
          <Button
            width={46}
            height={46}
            borderRadius={23}
            borderWidth={1}
            borderColor="rgba(120,120,128,0.32)"
            backgroundColor="rgba(120,120,128,0.10)"
            alignItems="center"
            justifyContent="center"
            pressStyle={{ scale: 0.97, opacity: 0.9 }}
            onPress={() => router.back()}
            accessibilityLabel="Back"
          >
            <Text fontSize="$7" lineHeight={24} fontWeight="700" marginTop={-1}>
              ‹
            </Text>
          </Button>
        }
        title="🍃 Energy Impact"
        subtitle="How the dashboard estimates today-so-far avoided emissions"
        right={(
          <YStack alignItems="flex-end">
            <AppMenu />
          </YStack>
        )}
      />

      <ScrollView style={{ flex: 1 }} contentContainerStyle={{ paddingBottom: 16 }} showsVerticalScrollIndicator>
        <YStack gap="$4">
          <Card gap="$2">
            <Text fontSize="$5" fontWeight="700">
              What this widget means
            </Text>
            <Text opacity={0.82}>
              Energy Impact estimates the pollution your measured solar generation has displaced on the grid today so far.
            </Text>
            <Text opacity={0.68}>
              The current dashboard uses real solar generation already recorded today. It does not annualize, extrapolate, or invent lifetime totals. Pollutant rows use grid-avoided emissions factors; the tree row uses a separate lifecycle-CO2 comparison.
            </Text>
          </Card>

          <Card
            gap="$2"
            style={{ borderColor: highlightBorder(typeof focus === 'string' ? focus : undefined, 'co2e') }}
          >
            <Text fontSize="$5" fontWeight="700">
              Pollutants shown
            </Text>
            <Text opacity={0.82}>CO2e: climate-warming emissions displaced by solar generation.</Text>
            <Text opacity={0.82}>NOx: nitrogen oxides, a smog-forming pollutant displaced by solar generation.</Text>
            <Text opacity={0.82}>SO2: sulfur dioxide displaced by solar generation.</Text>
          </Card>

          <Card gap="$2">
            <Text fontSize="$5" fontWeight="700">
              Current default factor
            </Text>
            <Text opacity={0.82}>
              The shipped default is {nyup.label} ({nyup.key}) using {nyup.source}.
            </Text>
            <Text opacity={0.68}>
              Factor version: {AVOIDED_EMISSIONS_FACTOR_VERSION}. A U.S. average fallback ({usAverage.key}) is defined for later ZIP/subregion mapping work.
            </Text>
          </Card>

          <Card
            gap="$2"
            style={{ borderColor: highlightBorder(typeof focus === 'string' ? focus : undefined, 'nox') }}
          >
            <Text fontSize="$5" fontWeight="700">
              Formula
            </Text>
            <Text opacity={0.82}>solarKWh = solarWh / 1000</Text>
            <Text opacity={0.82}>avoidedCO2e = solarKWh * {nyup.co2eKgPerKWh} kg/kWh</Text>
            <Text opacity={0.82}>avoidedNOx = solarKWh * {nyup.noxGramsPerKWh} g/kWh</Text>
            <Text opacity={0.82}>avoidedSO2 = solarKWh * {nyup.so2GramsPerKWh} g/kWh</Text>
          </Card>

          <Card
            gap="$3"
            style={{ borderColor: highlightBorder(typeof focus === 'string' ? focus : undefined, 'trees') }}
          >
            <Text fontSize="$5" fontWeight="700">
              Tree equivalent
            </Text>
            <Text opacity={0.82}>
              Tree equivalent compares solar lifecycle CO2, not avoided NOx or SO2.
            </Text>
            <Text opacity={0.82}>solarLifecycleCO2eKg = solarKWh * {PV_LIFECYCLE_CO2E_KG_PER_KWH}</Text>
            <Text opacity={0.82}>
              treeYearsEquivalent = solarLifecycleCO2eKg / {GENERIC_TREE_CO2_REMOVED_KG_PER_YEAR}
            </Text>
            <Text opacity={0.68}>
              Conservative benchmark: 1 mature tree-year ≈ {Math.round(GENERIC_TREE_KWH_PER_YEAR)} kWh. Factor version: {TREE_EQUIVALENT_FACTOR_VERSION}.
            </Text>
          </Card>

          <Card
            gap="$3"
            style={{ borderColor: highlightBorder(typeof focus === 'string' ? focus : undefined, 'so2') }}
          >
            <Text fontSize="$5" fontWeight="700">
              Full reference
            </Text>
            <Text opacity={0.8}>
              The full formulas, worked examples, constants, and implementation notes are stored in the repo reference docs.
            </Text>
            <XStack gap="$2" flexWrap="wrap">
              <Button
                size="$4"
                borderRadius="$5"
                backgroundColor="rgba(255,159,10,0.14)"
                borderColor="rgba(255,159,10,0.34)"
                borderWidth={1}
                onPress={() => openReferenceDoc(ENERGY_IMPACT_REFERENCE_DOC_URL)}
              >
                Open emissions doc
              </Button>
              <Button
                size="$4"
                borderRadius="$5"
                backgroundColor="rgba(16,185,129,0.14)"
                borderColor="rgba(16,185,129,0.34)"
                borderWidth={1}
                onPress={() => openReferenceDoc(TREE_EQUIVALENT_REFERENCE_DOC_URL)}
              >
                Open tree doc
              </Button>
            </XStack>
          </Card>
        </YStack>
      </ScrollView>
    </YStack>
  );
}
