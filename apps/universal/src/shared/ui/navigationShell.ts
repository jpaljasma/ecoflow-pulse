import { useMemo } from 'react';
import { useWindowDimensions } from 'react-native';
import { useNavigationShellStore } from './navigationShellStore';
import {
  NAVIGATION_SIDEBAR_BREAKPOINT,
  resolveCenteredPageMaxWidth,
  resolveCollapsedSidebarWidth,
  resolveExpandedSidebarWidth,
  resolvePageHorizontalPadding,
  resolvePageMaxWidth
} from './navigationShellMetrics';

export {
  NAVIGATION_SIDEBAR_BREAKPOINT,
  PULSE_PAGE_CONTENT_BOTTOM_PADDING,
  PULSE_CENTERED_PAGE_MAX_WIDTH,
  PULSE_OPERATING_PAGE_MAX_WIDTH,
  PULSE_PAGE_SECTION_GAP,
  PULSE_PANEL_COMPACT_PADDING,
  PULSE_PANEL_PADDING,
  PULSE_PANEL_RADIUS,
  resolveCenteredPageMaxWidth,
  resolveCollapsedSidebarWidth,
  resolveExpandedSidebarWidth,
  resolvePageHorizontalPadding,
  resolvePageHorizontalPaddingPx,
  resolvePageMaxWidth
} from './navigationShellMetrics';

type PulsePageLayoutVariant = 'operating' | 'centered';

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

export function usePageLayoutMetrics(variant: PulsePageLayoutVariant = 'operating') {
  const navigationMetrics = useNavigationShellMetrics();

  return useMemo(() => {
    const { contentWidth } = navigationMetrics;
    const resolveMaxWidth = variant === 'centered'
      ? resolveCenteredPageMaxWidth
      : resolvePageMaxWidth;

    return {
      ...navigationMetrics,
      compactHeader: contentWidth < 430,
      isTablet: contentWidth >= 768,
      isDesktop: contentWidth >= 1120,
      horizontalPadding: resolvePageHorizontalPadding(contentWidth),
      layoutMaxWidth: resolveMaxWidth(contentWidth)
    };
  }, [navigationMetrics, variant]);
}
