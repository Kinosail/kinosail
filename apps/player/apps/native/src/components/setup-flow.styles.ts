import { Platform, StyleSheet } from 'react-native';

import { radius, spacing } from '@/design/tokens';

const tv = Platform.isTV;

export const setupFlowStyles = StyleSheet.create({
  screen: { flex: 1 },
  scroll: {
    alignItems: 'stretch',
    flexGrow: 1,
    gap: tv ? 80 : spacing.six,
    justifyContent: 'center',
    padding: tv ? 64 : spacing.four,
  },
  scrollCompact: { gap: spacing.three, padding: spacing.three },
  intro: { alignSelf: 'center', gap: spacing.three, maxWidth: tv ? 760 : 640 },
  introCompact: { gap: spacing.two },
  eyebrow: {
    fontSize: tv ? 20 : 12,
    fontWeight: '900',
    letterSpacing: 2.5,
    marginTop: spacing.four,
  },
  eyebrowCompact: { marginTop: spacing.one },
  title: {
    fontSize: tv ? 72 : 48,
    fontWeight: '900',
    letterSpacing: -1.5,
    lineHeight: tv ? 78 : 51,
    maxWidth: tv ? 720 : 540,
  },
  titleCompact: { fontSize: 40, lineHeight: 43 },
  summary: {
    fontSize: tv ? 28 : 19,
    lineHeight: tv ? 40 : 29,
    maxWidth: tv ? 720 : 540,
  },
  rule: { height: 1, maxWidth: tv ? 720 : 540 },
  note: {
    fontSize: tv ? 22 : 13,
    lineHeight: tv ? 32 : 20,
    maxWidth: tv ? 680 : 480,
  },
  panel: {
    alignSelf: 'center',
    borderRadius: radius.panel,
    borderWidth: 1,
    flexBasis: tv ? 720 : 460,
    gap: spacing.three,
    maxWidth: tv ? 720 : 520,
    padding: tv ? 64 : spacing.four,
    width: '100%',
  },
  panelCompact: { flexBasis: 'auto', padding: spacing.three },
  step: { fontSize: tv ? 20 : 11, fontWeight: '900', letterSpacing: 1.8 },
  form: { gap: tv ? 24 : spacing.two },
  challenge: { gap: tv ? 28 : spacing.three },
  panelTitle: {
    fontSize: tv ? 40 : 28,
    fontWeight: '800',
    letterSpacing: -0.6,
  },
  body: { fontSize: tv ? 28 : 16, lineHeight: tv ? 40 : 24 },
  label: { fontSize: tv ? 24 : 13, fontWeight: '800', marginTop: spacing.one },
  input: {
    borderRadius: radius.control,
    borderWidth: 1,
    fontSize: tv ? 28 : 16,
    minHeight: tv ? 80 : 52,
    paddingHorizontal: spacing.two,
  },
  error: {
    fontSize: tv ? 24 : 14,
    fontWeight: '700',
    lineHeight: tv ? 34 : 20,
  },
  code: {
    fontSize: tv ? 72 : 48,
    fontVariant: ['tabular-nums'],
    fontWeight: '900',
    letterSpacing: 6,
    marginVertical: spacing.two,
  },
  waiting: { fontSize: tv ? 24 : 14, fontWeight: '700' },
  codeCompact: { fontSize: 40, letterSpacing: spacing.fine },
});
