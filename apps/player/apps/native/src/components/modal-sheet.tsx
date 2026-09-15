import React, { type ReactNode } from 'react';
import {
  Keyboard,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { useReducedMotion } from './use-reduced-motion';

export function ModalSheet({
  visible = true,
  animationType = 'slide',
  title,
  onClose,
  children,
  footer,
  dismissLabel = 'Close',
  dismissDisabled = false,
}: {
  visible?: boolean;
  animationType?: 'none' | 'slide' | 'fade';
  title: string;
  onClose(): void;
  children: ReactNode;
  footer?: ReactNode;
  dismissLabel?: string;
  dismissDisabled?: boolean;
}) {
  const theme = useKinoTheme();
  const reducedMotion = useReducedMotion();
  const close = () => {
    if (dismissDisabled) return;
    Keyboard.dismiss();
    onClose();
  };
  return (
    <Modal
      visible={visible}
      transparent
      animationType={reducedMotion ? 'none' : animationType}
      supportedOrientations={[
        'portrait',
        'portrait-upside-down',
        'landscape-left',
        'landscape-right',
      ]}
      onRequestClose={close}
    >
      <KeyboardAvoidingView
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        style={styles.backdrop}
      >
        <Pressable
          accessible={false}
          focusable={false}
          importantForAccessibility="no"
          onPress={close}
          style={StyleSheet.absoluteFill}
        />
        <SafeAreaView
          edges={['top', 'left', 'right']}
          pointerEvents="box-none"
          style={styles.space}
        >
          <SafeAreaView
            edges={['bottom']}
            accessibilityViewIsModal
            onAccessibilityEscape={close}
            style={[styles.sheet, { backgroundColor: theme.surface }]}
          >
            <View style={styles.heading}>
              <Text
                accessibilityRole="header"
                numberOfLines={2}
                style={[styles.title, { color: theme.text }]}
              >
                {title}
              </Text>
              {Platform.isTV ? (
                <ActionButton
                  label={dismissLabel}
                  disabled={dismissDisabled}
                  preferredFocus
                  quiet
                  onPress={close}
                />
              ) : null}
            </View>
            <ScrollView
              style={styles.scroll}
              contentContainerStyle={styles.content}
              keyboardShouldPersistTaps="handled"
              keyboardDismissMode="on-drag"
            >
              {children}
            </ScrollView>
            {footer || !Platform.isTV ? (
              <View style={[styles.footer, { borderTopColor: theme.line }]}>
                {footer ? (
                  <View style={{ flex: 1, minWidth: 180, maxWidth: '100%' }}>
                    {footer}
                  </View>
                ) : null}
                {!Platform.isTV ? (
                  <ActionButton
                    label={dismissLabel}
                    disabled={dismissDisabled}
                    preferredFocus
                    quiet
                    onPress={close}
                  />
                ) : null}
              </View>
            ) : null}
          </SafeAreaView>
        </SafeAreaView>
      </KeyboardAvoidingView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, backgroundColor: '#00000066' },
  space: { flex: 1, justifyContent: Platform.isTV ? 'center' : 'flex-end' },
  sheet: {
    maxHeight: '90%',
    flexShrink: 1,
    width: '100%',
    maxWidth: Platform.isTV ? 1040 : 560,
    alignSelf: 'center',
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
  },
  heading: {
    flexDirection: 'row',
    alignItems: 'center',
    flexShrink: 0,
    padding: 20,
    gap: 12,
  },
  title: { flex: 1, fontSize: Platform.isTV ? 36 : 24, fontWeight: '700' },
  scroll: { flexShrink: 1, flexGrow: 0 },
  content: { paddingHorizontal: 20, paddingBottom: 20, gap: 16 },
  footer: {
    flexShrink: 0,
    padding: 12,
    borderTopWidth: 1,
    flexDirection: 'row',
    flexWrap: 'wrap',
    justifyContent: 'flex-end',
    alignItems: 'center',
    gap: 12,
  },
});
