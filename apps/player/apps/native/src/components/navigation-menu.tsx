import React, { useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { downloadsAvailable } from '@/core/downloads';
import {
  defaultNavigation,
  navigationDestinations,
  type NavigationDestination,
} from '@/core/navigation-preferences';
import { useKinoTheme } from '@/design/tokens';
import { ActionButton } from './action-button';
import { NavigationIcon } from './navigation-icon';
import { ModalSheet } from './modal-sheet';

type Props = {
  panel: 'closed' | 'more' | 'edit';
  tabs: NavigationDestination[];
  draft: NavigationDestination[];
  loaded: boolean;
  saving: boolean;
  error: string;
  onClose(): void;
  onEdit(): void;
  onChange(tabs: NavigationDestination[]): void;
  onSave(): void;
  onNavigate(destination: NavigationDestination): void;
};
const available = (
  Object.keys(navigationDestinations) as NavigationDestination[]
).filter((destination) => downloadsAvailable || destination !== 'downloads');
function EditControl({
  icon,
  label,
  disabled,
  onPress,
}: {
  icon: 'up' | 'down' | 'add' | 'remove';
  label: string;
  disabled: boolean;
  onPress(): void;
}) {
  const theme = useKinoTheme();
  const [focused, setFocused] = useState(false);
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityState={{ disabled }}
      disabled={disabled}
      focusable={!disabled}
      onPress={onPress}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      style={({ pressed }) => [
        styles.control,
        {
          opacity: disabled ? 0.35 : pressed ? 0.65 : 1,
          borderColor: focused ? theme.focus : 'transparent',
        },
      ]}
    >
      <NavigationIcon name={icon} color={theme.text} />
    </Pressable>
  );
}
export function NavigationMenu(props: Props) {
  const theme = useKinoTheme();
  const editing = props.panel === 'edit';
  const hidden = available.filter(
    (destination) =>
      !(editing ? props.draft : props.tabs).includes(destination),
  );
  const move = (index: number, direction: number) => {
    const next = [...props.draft];
    [next[index], next[index + direction]] = [
      next[index + direction],
      next[index],
    ];
    props.onChange(next);
  };
  return (
    <ModalSheet
      visible={props.panel !== 'closed'}
      title={editing ? 'Customize tabs' : 'More'}
      onClose={props.onClose}
      dismissLabel={editing ? 'Cancel' : 'Done'}
      dismissDisabled={props.saving}
      footer={
        editing ? (
          <ActionButton
            label="Save"
            busy={props.saving}
            onPress={props.onSave}
            style={{ flex: 1 }}
          />
        ) : undefined
      }
    >
      {editing ? (
        <Text style={{ color: theme.muted }}>
          Choose up to four tabs. More stays available.
        </Text>
      ) : null}
      {editing ? (
        <>
          {props.draft.map((destination, index) => (
            <View
              key={destination}
              style={[styles.row, { borderBottomColor: theme.line }]}
            >
              <Text style={[styles.name, { color: theme.text }]}>
                {navigationDestinations[destination]}
              </Text>
              <EditControl
                icon="up"
                label={`Move ${navigationDestinations[destination]} earlier`}
                disabled={props.saving || index === 0}
                onPress={() => move(index, -1)}
              />
              <EditControl
                icon="down"
                label={`Move ${navigationDestinations[destination]} later`}
                disabled={props.saving || index === props.draft.length - 1}
                onPress={() => move(index, 1)}
              />
              <EditControl
                icon="remove"
                label={`Remove ${navigationDestinations[destination]} from tab bar`}
                disabled={props.saving || props.draft.length === 1}
                onPress={() =>
                  props.onChange(
                    props.draft.filter((tab) => tab !== destination),
                  )
                }
              />
            </View>
          ))}
          <Text
            accessibilityRole="header"
            style={[styles.section, { color: theme.text }]}
          >
            Available in More
          </Text>
          <Text style={{ color: theme.muted }} accessibilityLiveRegion="polite">
            {props.draft.length === 4
              ? 'Remove a tab to add another.'
              : `${4 - props.draft.length} ${props.draft.length === 3 ? 'space' : 'spaces'} available.`}
          </Text>
          {hidden.map((destination) => (
            <View
              key={destination}
              style={[styles.row, { borderBottomColor: theme.line }]}
            >
              <Text style={[styles.name, { color: theme.text }]}>
                {navigationDestinations[destination]}
              </Text>
              <EditControl
                icon="add"
                label={`Add ${navigationDestinations[destination]} to tab bar`}
                disabled={props.saving || props.draft.length === 4}
                onPress={() => props.onChange([...props.draft, destination])}
              />
            </View>
          ))}
          <ActionButton
            label="Restore defaults"
            quiet
            disabled={props.saving}
            onPress={() =>
              props.onChange(defaultNavigation(downloadsAvailable))
            }
          />
        </>
      ) : (
        <>
          {hidden.map((destination) => (
            <ActionButton
              key={destination}
              label={navigationDestinations[destination]}
              quiet
              onPress={() => props.onNavigate(destination)}
            />
          ))}
          <ActionButton
            label="Customize tabs"
            onPress={props.onEdit}
            disabled={!props.loaded}
          />
        </>
      )}
      {props.error ? (
        <Text accessibilityRole="alert" style={{ color: theme.text }}>
          {props.error}
        </Text>
      ) : null}
    </ModalSheet>
  );
}
const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    minHeight: 56,
    borderBottomWidth: 1,
  },
  name: { flex: 1, fontSize: 16, fontWeight: '600' },
  control: {
    width: 44,
    minHeight: 48,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderRadius: 8,
  },
  section: { fontSize: 17, fontWeight: '700', paddingTop: 16 },
  footer: { flexDirection: 'row', flexWrap: 'wrap', gap: 12 },
});
