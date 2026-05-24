export const NAVIGATION_SIDEBAR_BREAKPOINT = 1024;
export const PULSE_PAGE_SECTION_GAP = 16;
export const PULSE_PAGE_CONTENT_BOTTOM_PADDING = 16;
export const PULSE_PANEL_RADIUS = '$1' as const;
export const PULSE_PANEL_PADDING = '$6' as const;
export const PULSE_PANEL_COMPACT_PADDING = '$4' as const;
export const PULSE_OPERATING_PAGE_MAX_WIDTH = 1180;
export const PULSE_CENTERED_PAGE_MAX_WIDTH = 1040;

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

export function resolvePageHorizontalPadding(contentWidth: number) {
  if (contentWidth >= 1120) {
    return '$6' as const;
  }
  if (contentWidth >= 768) {
    return '$5' as const;
  }
  return '$4' as const;
}

export function resolvePageHorizontalPaddingPx(contentWidth: number): number {
  if (contentWidth >= 1120) {
    return 24;
  }
  if (contentWidth >= 768) {
    return 20;
  }
  return 16;
}

export function resolvePageMaxWidth(contentWidth: number) {
  return contentWidth >= 1120 ? PULSE_OPERATING_PAGE_MAX_WIDTH : 980;
}

export function resolveCenteredPageMaxWidth(contentWidth: number) {
  return contentWidth >= 1120 ? PULSE_CENTERED_PAGE_MAX_WIDTH : 920;
}
