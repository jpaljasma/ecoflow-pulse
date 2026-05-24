import { describe, expect, it } from 'vitest';
import {
  PULSE_ACTION_BUTTON_HOVER_TRANSITION_MS,
  PULSE_ACTION_BUTTON_PRESS_SCALE,
  buildPulseActionButtonStyles
} from '@/shared/ui/buttonInteractions';

const semantics = {
  actionBackground: 'rgba(24, 144, 255, 0.14)',
  actionBorder: 'rgba(24, 144, 255, 0.34)',
  actionHoverBackground: 'rgba(24, 144, 255, 0.22)',
  actionPressBackground: 'rgba(24, 144, 255, 0.28)'
};

describe('buildPulseActionButtonStyles', () => {
  it('uses the shared accent color for hover and press borders', () => {
    const styles = buildPulseActionButtonStyles(semantics, { web: false });

    expect(styles.hoverStyle.borderColor).toBe('$accentColor');
    expect(styles.pressStyle.borderColor).toBe('$accentColor');
  });

  it('adds a CSS transition for web buttons', () => {
    const styles = buildPulseActionButtonStyles(semantics, { web: true });

    expect(styles.style.transitionDuration).toBe(`${PULSE_ACTION_BUTTON_HOVER_TRANSITION_MS}ms`);
    expect(styles.style.transitionProperty).toContain('background-color');
    expect(styles.pressStyle.scale).toBe(PULSE_ACTION_BUTTON_PRESS_SCALE);
  });
});
