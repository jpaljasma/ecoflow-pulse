import { create } from 'zustand';
import { Platform } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { createJSONStorage, persist } from 'zustand/middleware';

export type TrendChartStyle = 'premium' | 'ascii';

type ChartPrefsState = {
  trendChartStyle: TrendChartStyle;
  toggleTrendChartStyle: () => void;
  setTrendChartStyle: (style: TrendChartStyle) => void;
};

const defaultTrendChartStyle: TrendChartStyle = Platform.OS === 'web' ? 'ascii' : 'premium';

const storage = createJSONStorage(() => {
  if (Platform.OS === 'web') {
    return localStorage;
  }
  return AsyncStorage;
});

export const useChartPrefs = create<ChartPrefsState>()(
  persist(
    (set) => ({
      trendChartStyle: defaultTrendChartStyle,
      toggleTrendChartStyle: () =>
        set((state) => ({
          trendChartStyle: state.trendChartStyle === 'premium' ? 'ascii' : 'premium'
        })),
      setTrendChartStyle: (style) => set({ trendChartStyle: style })
    }),
    {
      name: 'ecoflow-pulse-chart-prefs',
      storage,
      partialize: (state) => ({
        trendChartStyle: state.trendChartStyle
      })
    }
  )
);
