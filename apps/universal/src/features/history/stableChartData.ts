import { useEffect, useMemo, useRef } from 'react';

export type StableChartDataSelection<T> = {
  data: T | undefined;
  lastGoodData: T | undefined;
  currentKey: string;
  lastGoodKey: string | undefined;
  isFetching: boolean;
  isError: boolean;
  isPlaceholderData: boolean;
  isUsable: (data: T) => boolean;
};

export function hasNonZeroSeriesValue(values: ReadonlyArray<number | null | undefined>): boolean {
  return values.some((value) => typeof value === 'number' && Number.isFinite(value) && value !== 0);
}

export function selectStableChartData<T>({
  data,
  lastGoodData,
  currentKey,
  lastGoodKey,
  isFetching,
  isError,
  isPlaceholderData,
  isUsable
}: StableChartDataSelection<T>): T | undefined {
  if (data !== undefined && isUsable(data)) {
    return data;
  }

  const canUseLastGood =
    lastGoodData !== undefined &&
    lastGoodKey === currentKey &&
    (isFetching || isError || isPlaceholderData);

  return canUseLastGood ? lastGoodData : data;
}

export function useStableChartData<T>({
  data,
  stableKey,
  isFetching,
  isError,
  isPlaceholderData,
  isSuccess,
  isUsable
}: {
  data: T | undefined;
  stableKey: string;
  isFetching: boolean;
  isError: boolean;
  isPlaceholderData: boolean;
  isSuccess: boolean;
  isUsable: (data: T) => boolean;
}): T | undefined {
  const lastGoodRef = useRef<{ key: string; data: T } | null>(null);

  useEffect(() => {
    if (isSuccess && !isPlaceholderData && data !== undefined && isUsable(data)) {
      lastGoodRef.current = { key: stableKey, data };
    }
  }, [data, isPlaceholderData, isSuccess, isUsable, stableKey]);

  return useMemo(
    () =>
      selectStableChartData({
        data,
        lastGoodData: lastGoodRef.current?.data,
        currentKey: stableKey,
        lastGoodKey: lastGoodRef.current?.key,
        isFetching,
        isError,
        isPlaceholderData,
        isUsable
      }),
    [data, isError, isFetching, isPlaceholderData, isUsable, stableKey]
  );
}
