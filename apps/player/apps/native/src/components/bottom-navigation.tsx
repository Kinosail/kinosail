import { router, useGlobalSearchParams, usePathname } from 'expo-router';
import React, { useEffect, useRef, useState } from 'react';
import {
  Keyboard,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { downloadsAvailable } from '@/core/downloads';
import { useSession } from '@/core/session-context';
import { NavigationIcon } from './navigation-icon';
import { useKinoTheme } from '@/design/tokens';
import { platformStorage } from '@/core/platform-storage';
import {
  createNavigationPreferences,
  defaultNavigation,
  navigationDestinations,
  type NavigationDestination,
} from '@/core/navigation-preferences';
import { NavigationMenu } from './navigation-menu';

const preferences = createNavigationPreferences(
  platformStorage,
  downloadsAvailable,
);

export function BottomNavigation() {
  const { client } = useSession();
  const pathname = usePathname();
  const { view } = useGlobalSearchParams();
  const theme = useKinoTheme();
  const insets = useSafeAreaInsets();
  const [keyboardVisible, setKeyboardVisible] = useState(false);
  const [focused, setFocused] = useState('');
  const [tabs, setTabs] = useState(() => defaultNavigation(downloadsAvailable));
  const [draft, setDraft] = useState(tabs);
  const [panel, setPanel] = useState<'closed' | 'more' | 'edit'>('closed');
  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const savingRef = useRef(false);
  useEffect(() => {
    let active = true;
    preferences
      .load()
      .then(
        (saved) => {
          if (active) setTabs(saved);
        },
        () => {
          if (active)
            setError('Saved tabs could not be read. Choose your tabs again.');
        },
      )
      .finally(() => {
        if (active) setLoaded(true);
      });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    setPanel('closed');
  }, [client, pathname]);
  const edit = () => {
    if (!loaded) return;
    setDraft([...tabs]);
    setPanel('edit');
  };
  const save = async () => {
    if (savingRef.current) return;
    savingRef.current = true;
    setSaving(true);
    setError('');
    try {
      const saved = await preferences.save(draft);
      setTabs(saved);
      setPanel('closed');
    } catch {
      setError('Your tabs could not be saved. Try again.');
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };
  const navigate = (destination: NavigationDestination) => {
    setPanel('closed');
    if (destination === 'home') router.replace('/');
    else if (destination === 'settings') router.replace('/settings');
    else if (destination === 'collections') router.replace('/collections');
    else if (destination === 'downloads') router.replace('/downloads');
    else
      router.replace({ pathname: '/library', params: { view: destination } });
  };
  useEffect(() => {
    const show = Keyboard.addListener('keyboardDidShow', () =>
      setKeyboardVisible(true),
    );
    const hide = Keyboard.addListener('keyboardDidHide', () =>
      setKeyboardVisible(false),
    );
    return () => {
      show.remove();
      hide.remove();
    };
  }, []);
  if (
    Platform.isTV ||
    !client ||
    keyboardVisible ||
    !['/', '/library', '/downloads', '/settings', '/collections'].includes(
      pathname,
    )
  )
    return null;
  const selected =
    pathname === '/'
      ? 'home'
      : pathname === '/settings'
        ? 'settings'
        : pathname === '/downloads'
          ? 'downloads'
          : pathname === '/collections'
            ? 'collections'
            : view;
  const activeTab = tabs.some((tab) => tab === selected) ? selected : 'more';
  const floating = pathname === '/' || pathname === '/library';
  return (
    <View
      testID="bottom-navigation-overlay"
      pointerEvents="box-none"
      style={
        floating
          ? { position: 'absolute', bottom: 0, left: 0, right: 0 }
          : undefined
      }
    >
      {floating ? (
        <View pointerEvents="box-none" style={{ height: 0, zIndex: 10 }}>
          <View
            pointerEvents="box-none"
            style={{
              position: 'absolute',
              bottom: 12,
              left: Math.max(insets.left, 20),
              right: Math.max(insets.right, 20),
              flexDirection: 'row',
              justifyContent: 'flex-end',
              alignItems: 'center',
            }}
          >
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Search your library"
              focusable
              onFocus={() => setFocused('search')}
              onBlur={() => setFocused('')}
              onPress={() =>
                router.navigate({
                  pathname: '/library',
                  params: {
                    search: '1',
                    ...(pathname === '/library' && typeof view === 'string'
                      ? { view }
                      : {}),
                  },
                })
              }
              style={{
                minWidth: 56,
                minHeight: 56,
                paddingHorizontal: 0,
                borderRadius: 28,
                borderWidth: 2,
                borderColor: focused === 'search' ? theme.focus : theme.line,
                backgroundColor: theme.surface,
                flexDirection: 'row',
                gap: 8,
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: '0 4px 16px #00000030',
              }}
            >
              <NavigationIcon name="search" color={theme.text} />
            </Pressable>
          </View>
        </View>
      ) : null}
      <View
        pointerEvents="box-none"
        role={Platform.OS === 'web' ? 'navigation' : undefined}
        accessibilityLabel="Media navigation"
        style={{
          backgroundColor: 'transparent',
          paddingHorizontal: 12,
          paddingBottom: Math.max(insets.bottom, 8),
          paddingTop: 4,
        }}
      >
        <View
          pointerEvents="box-none"
          role="tablist"
          accessibilityLabel="Media navigation"
          style={[
            styles.bar,
            {
              backgroundColor: theme.surface,
              borderColor: theme.line,
              paddingBottom: 4,
              paddingLeft: Math.max(insets.left, 4),
              paddingRight: Math.max(insets.right, 4),
            },
          ]}
        >
          {[...tabs, 'more' as const].map((destination) => (
            <Pressable
              key={destination}
              accessibilityRole="tab"
              focusable
              accessibilityLabel={
                destination === 'more'
                  ? 'More'
                  : navigationDestinations[destination]
              }
              accessibilityHint="Touch and hold to customize the tab bar."
              accessibilityState={{ selected: activeTab === destination }}
              aria-selected={activeTab === destination}
              onFocus={() => setFocused(destination)}
              onBlur={() => setFocused('')}
              onLongPress={edit}
              onPress={() => {
                if (destination === 'more') setPanel('more');
                else if (destination !== selected) navigate(destination);
              }}
              style={({ pressed }) => [
                styles.tab,
                {
                  borderColor:
                    focused === destination ? theme.focus : 'transparent',
                  backgroundColor:
                    pressed || activeTab === destination
                      ? theme.raised
                      : 'transparent',
                },
              ]}
            >
              <NavigationIcon
                name={destination === 'downloads' ? 'download' : destination}
                color={activeTab === destination ? theme.signal : theme.muted}
              />
              <Text
                style={[
                  styles.label,
                  {
                    color:
                      activeTab === destination ? theme.signal : theme.muted,
                  },
                ]}
              >
                {destination === 'more'
                  ? 'More'
                  : navigationDestinations[destination]}
              </Text>
            </Pressable>
          ))}
        </View>
      </View>
      <NavigationMenu
        panel={panel}
        tabs={tabs}
        draft={draft}
        loaded={loaded}
        saving={saving}
        error={error}
        onClose={() => setPanel('closed')}
        onEdit={edit}
        onChange={setDraft}
        onSave={() => void save()}
        onNavigate={navigate}
      />
    </View>
  );
}
const styles = StyleSheet.create({
  bar: {
    flexDirection: 'row',
    borderWidth: 1,
    borderRadius: 32,
    paddingTop: 4,
  },
  tab: {
    flex: 1,
    minWidth: 44,
    minHeight: 60,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: 1,
    paddingVertical: 6,
    gap: 5,
    borderWidth: 2,
    borderRadius: 26,
  },
  label: { fontSize: 12, fontWeight: '600', textAlign: 'center' },
});
