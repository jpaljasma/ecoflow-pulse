import { useMemo } from 'react';
import { useWindowDimensions } from 'react-native';
import { useNavigationShellStore } from './navigationShellStore';

export const NAVIGATION_SIDEBAR_BREAKPOINT = 1024;

export function resolveExpandedSidebarWidth(windowWidth: number): number {
  if (windowWidth >= 1480) {
    return 248;
  }
  if (windowWidth >= 1280) {
    return 236;
  }
  return 224;
}

export function resolveCollapsedSidebarWidth(windowWidth: number): number {
  return windowWidth >= 1480 ? 92 : 84;
}

export function useNavigationShellMetrics() {
  const { width: windowWidth, height: windowHeight } = useWindowDimensions();
  const sidebarExpanded = useNavigationShellStore((state) => state.sidebarExpanded);
  const hydrated = useNavigationShellStore((state) => state.hydrated);

  return useMemo(() => {
    const isSidebarMode = windowWidth >= NAVIGATION_SIDEBAR_BREAKPOINT;
    const sidebarWidth = isSidebarMode
      ? sidebarExpanded
        ? resolveExpandedSidebarWidth(windowWidth)
        : resolveCollapsedSidebarWidth(windowWidth)
      : 0;
    const contentWidth = Math.max(320, windowWidth - sidebarWidth);

    return {
      hydrated,
      windowWidth,
      windowHeight,
      contentWidth,
      isSidebarMode,
      sidebarExpanded,
      sidebarWidth
    };
  }, [hydrated, sidebarExpanded, windowHeight, windowWidth]);
}

export function resolvePageHorizontalPadding(contentWidth: number) {
  if (contentWidth >= 1120) {
    return '$7' as const;
  }
  if (contentWidth >= 768) {
    return '$5' as const;
  }
  return '$4' as const;
}

export function resolvePageMaxWidth(contentWidth: number) {
  return contentWidth >= 1120 ? 1180 : 980;
}

export function usePageLayoutMetrics() {
  const navigationMetrics = useNavigationShellMetrics();

  return useMemo(() => {
    const { contentWidth } = navigationMetrics;
    return {
      ...navigationMetrics,
      compactHeader: contentWidth < 430,
      isTablet: contentWidth >= 768,
      isDesktop: contentWidth >= 1120,
      horizontalPadding: resolvePageHorizontalPadding(contentWidth),
      layoutMaxWidth: resolvePageMaxWidth(contentWidth)
    };
  }, [navigationMetrics]);
}
