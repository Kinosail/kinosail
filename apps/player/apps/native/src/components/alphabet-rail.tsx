import React, { useRef, useState } from 'react';
import { Platform, StyleSheet, Text, View, type ViewStyle } from 'react-native';
import type { LibraryLetter } from '@/core/contract';
import { useKinoTheme } from '@/design/tokens';

type Props = {
  letters: LibraryLetter[];
  offset: number;
  onJump(offset: number): void;
};
export function AlphabetRail({ letters, offset, onJump }: Props) {
  const theme = useKinoTheme();
  const rail = useRef<View>(null);
  const bounds = useRef({ top: 0, height: 1 });
  const selected = Math.max(
    0,
    letters.reduce(
      (current, letter, index) => (letter.offset <= offset ? index : current),
      0,
    ),
  );
  const [dragging, setDragging] = useState<number | null>(null);
  const dragIndex = useRef<number | null>(null);
  const active = dragging ?? selected;
  const move = (pageY: number) => {
    const index = Math.max(
      0,
      Math.min(
        letters.length - 1,
        Math.floor(
          ((pageY - bounds.current.top) / bounds.current.height) *
            letters.length,
        ),
      ),
    );
    dragIndex.current = index;
    setDragging(index);
  };
  const jump = (index: number) =>
    onJump(letters[Math.max(0, Math.min(letters.length - 1, index))].offset);
  return (
    <View style={styles.position} pointerEvents="box-none">
      <View
        ref={rail}
        accessible
        focusable
        accessibilityRole="adjustable"
        accessibilityLabel="Library alphabet"
        accessibilityHint="Swipe up or down to jump to a letter."
        accessibilityValue={{
          min: 0,
          max: letters.length - 1,
          now: active,
          text: letters[active].label,
        }}
        accessibilityActions={[
          { name: 'increment', label: 'Next letter' },
          { name: 'decrement', label: 'Previous letter' },
        ]}
        onAccessibilityAction={({ nativeEvent }) => {
          if (nativeEvent.actionName === 'increment') jump(selected + 1);
          if (nativeEvent.actionName === 'decrement') jump(selected - 1);
        }}
        {...(Platform.OS === 'web'
          ? {
              'aria-orientation': 'vertical',
              'aria-valuemin': 0,
              'aria-valuemax': letters.length - 1,
              'aria-valuenow': active,
              'aria-valuetext': letters[active].label,
              onKeyDown: (event: { key: string; preventDefault(): void }) => {
                const index =
                  event.key === 'ArrowDown' || event.key === 'ArrowRight'
                    ? selected + 1
                    : event.key === 'ArrowUp' || event.key === 'ArrowLeft'
                      ? selected - 1
                      : event.key === 'Home'
                        ? 0
                        : event.key === 'End'
                          ? letters.length - 1
                          : null;
                if (index !== null) {
                  event.preventDefault();
                  jump(index);
                }
              },
            }
          : {})}
        onLayout={() =>
          rail.current?.measureInWindow((_x, top, _width, height) => {
            bounds.current = { top, height: Math.max(1, height) };
          })
        }
        onStartShouldSetResponder={() => true}
        onMoveShouldSetResponder={() => true}
        onResponderGrant={(event) => move(event.nativeEvent.pageY)}
        onResponderMove={(event) => move(event.nativeEvent.pageY)}
        onResponderRelease={() => {
          if (dragIndex.current !== null) jump(dragIndex.current);
          dragIndex.current = null;
          setDragging(null);
        }}
        onResponderTerminationRequest={() => false}
        onResponderTerminate={() => {
          dragIndex.current = null;
          setDragging(null);
        }}
        style={[
          styles.rail,
          Platform.OS === 'web'
            ? ({ touchAction: 'none' } as ViewStyle & { touchAction: 'none' })
            : null,
        ]}
      >
        {letters.map((letter, index) =>
          letters.length > 32 &&
          index !== active &&
          index % Math.ceil(letters.length / 26) !== 0 ? null : (
            <View
              key={letter.label}
              accessible={false}
              pointerEvents="none"
              style={[
                styles.letter,
                {
                  top: `${(index / letters.length) * 100}%`,
                  height: `${100 / letters.length}%`,
                },
              ]}
            >
              <Text
                accessible={false}
                allowFontScaling={false}
                style={{
                  color: index === active ? theme.signal : theme.muted,
                  fontSize: 11,
                  fontWeight: index === active ? '800' : '600',
                }}
              >
                {letter.label}
              </Text>
            </View>
          ),
        )}
        {dragging !== null ? (
          <View
            pointerEvents="none"
            style={[styles.preview, { backgroundColor: theme.surface }]}
          >
            <Text
              style={{ color: theme.signal, fontSize: 36, fontWeight: '700' }}
            >
              {letters[active].label}
            </Text>
          </View>
        ) : null}
      </View>
    </View>
  );
}
const styles = StyleSheet.create({
  position: {
    position: 'absolute',
    right: 0,
    top: 0,
    bottom: 100,
    width: 44,
    justifyContent: 'center',
  },
  rail: {
    width: 44,
    height: '100%',
    maxHeight: 520,
    justifyContent: 'space-around',
  },
  letter: {
    position: 'absolute',
    width: '100%',
    alignItems: 'center',
    justifyContent: 'center',
  },
  preview: {
    position: 'absolute',
    right: 52,
    top: '40%',
    minWidth: 64,
    minHeight: 64,
    borderRadius: 16,
    alignItems: 'center',
    justifyContent: 'center',
  },
});
