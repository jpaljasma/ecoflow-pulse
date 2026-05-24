export const PULSE_ACTION_BUTTON_HOVER_TRANSITION_MS = 160;
export const PULSE_ACTION_BUTTON_PRESS_SCALE = 0.985;

export type PulseActionButtonSemantics = {
  actionBackground: string;
  actionBorder: string;
  actionHoverBackground: string;
  actionPressBackground: string;
};

type PulseActionButtonStyle = Record<string, any>;

export type PulseActionButtonStyles = {
  style: PulseActionButtonStyle;
  hoverStyle: PulseActionButtonStyle;
  pressStyle: PulseActionButtonStyle;
};

export function buildPulseActionButtonStyles(
  semantics: PulseActionButtonSemantics,
  { web = false }: { web?: boolean } = {}
): PulseActionButtonStyles {
  return {
    style: {
      backgroundColor: semantics.actionBackground,
      borderColor: semantics.actionBorder,
      ...(web
        ? {
            transitionProperty: 'background-color, border-color, box-shadow, transform',
            transitionDuration: `${PULSE_ACTION_BUTTON_HOVER_TRANSITION_MS}ms`,
            transitionTimingFunction: 'ease-out'
          }
        : {})
    },
    hoverStyle: {
      backgroundColor: semantics.actionHoverBackground,
      borderColor: '$accentColor',
      transform: [{ translateY: -1 }],
      shadowOpacity: 0.12
    },
    pressStyle: {
      backgroundColor: semantics.actionPressBackground,
      borderColor: '$accentColor',
      scale: PULSE_ACTION_BUTTON_PRESS_SCALE,
      opacity: 0.92
    }
  };
}
