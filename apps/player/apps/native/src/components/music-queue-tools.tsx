import React, { useState } from 'react';
import { Text, View } from 'react-native';
import type { MusicQueue } from '@/core/use-music-queue';
import { useKinoTheme } from '@/design/tokens';
import { AudioButton } from './audio-player-controls';
import { ActionButton } from './action-button';
import { ModalSheet } from './modal-sheet';

export function MusicQueueTools({
  queue,
  onSelect,
}: {
  queue: MusicQueue;
  onSelect(id: string): void;
}) {
  const theme = useKinoTheme();
  const [page, setPage] = useState<number | null>(null);
  const repeatLabel =
    queue.repeat === 'one' ? 'Track' : queue.repeat === 'all' ? 'Album' : 'Off';
  return (
    <>
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-evenly',
          paddingHorizontal: 16,
        }}
      >
        <AudioButton
          icon="shuffle"
          caption="Shuffle"
          label={`Shuffle ${queue.shuffled ? 'on' : 'off'}`}
          selected={queue.shuffled}
          disabled={queue.items.length < 2}
          onPress={queue.shuffle}
        />
        <AudioButton
          icon="repeat"
          caption={`Repeat: ${repeatLabel}`}
          label={`Repeat ${repeatLabel.toLowerCase()}`}
          selected={queue.repeat !== 'off'}
          disabled={!queue.items.length}
          onPress={queue.cycleRepeat}
        />
        <AudioButton
          icon="list"
          caption="Queue"
          label={`Queue, ${queue.items.length} tracks`}
          disabled={!queue.items.length}
          onPress={() => setPage(Math.floor(Math.max(0, queue.index) / 50))}
        />
      </View>
      <Text
        accessibilityLiveRegion="polite"
        style={{
          color: theme.muted,
          paddingHorizontal: 24,
          paddingVertical: 8,
        }}
      >
        {queue.loading
          ? 'Loading album queue…'
          : queue.error ||
            (queue.index >= 0
              ? `Track ${queue.index + 1} of ${queue.items.length}`
              : '')}
      </Text>
      {queue.error ? (
        <ActionButton label="Retry queue" quiet onPress={queue.retry} />
      ) : null}
      {page !== null ? (
        <ModalSheet title="Play queue" onClose={() => setPage(null)}>
          {queue.items.slice(page * 50, (page + 1) * 50).map((item, index) => (
            <ActionButton
              key={item.id}
              label={`${page * 50 + index + 1}. ${item.title}${item.artist ? ` · ${item.artist}` : ''}`}
              selected={page * 50 + index === queue.index}
              quiet={page * 50 + index !== queue.index}
              onPress={() => {
                setPage(null);
                onSelect(item.id);
              }}
            />
          ))}
          {page > 0 ? (
            <ActionButton
              label="Previous tracks"
              quiet
              onPress={() => setPage(page - 1)}
            />
          ) : null}
          {(page + 1) * 50 < queue.items.length ? (
            <ActionButton
              label="More tracks"
              quiet
              onPress={() => setPage(page + 1)}
            />
          ) : null}
        </ModalSheet>
      ) : null}
    </>
  );
}
