import { Image } from 'expo-image';
import React from 'react';
import { svgDataURI } from '@/core/svg-data-uri';

const paths = {
  shuffle:
    '<path d="M3 5h3l12 14h3M3 19h3L18 5h3M17 2l4 3-4 3M17 16l4 3-4 3"/>',
  repeat:
    '<path d="M4 8h14l-3-3M20 16H6l3 3M18 8l-3 3M6 16l3-3M4 8v4M20 16v-4"/>',
  photos:
    '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8" cy="8" r="2"/><path d="m3 18 6-6 4 4 3-5 5 7"/>',
  collections: '<path d="M3 7h7l2 3h9v11H3ZM3 7V3h7l2 3h9v4"/>',
  audiobooks: '<path d="M3 14v-3a9 9 0 0 1 18 0v3M3 12h4v9H3ZM17 12h4v9h-4Z"/>',
  list: '<path d="M8 5h13M8 12h13M8 19h13M3 5h.1M3 12h.1M3 19h.1"/>',
  history: '<circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/>',
  airplay: '<path d="M5 17H3V3h18v14h-2M12 14l6 7H6Z"/>',
  settings:
    '<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="8" cy="6" r="2"/><circle cx="16" cy="12" r="2"/><circle cx="10" cy="18" r="2"/>',
  play: '<path d="m8 4 12 8-12 8Z" fill="currentColor"/>',
  back30:
    '<path d="M5 8a8 8 0 1 1-1 8M5 3v5h5"/><text x="12" y="17" fill="currentColor" stroke="none" text-anchor="middle" font-family="sans-serif" font-size="9" font-weight="700">30</text>',
  forward30:
    '<path d="M19 8a8 8 0 1 0 1 8M19 3v5h-5"/><text x="12" y="17" fill="currentColor" stroke="none" text-anchor="middle" font-family="sans-serif" font-size="9" font-weight="700">30</text>',
  bookmark: '<path d="M6 3h12v18l-6-4-6 4Z"/>',
  sleep: '<path d="M20 15A9 9 0 0 1 9 4a9 9 0 1 0 11 11Z"/>',
  speed: '<circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/>',
  chapters: '<path d="M8 5h13M8 12h13M8 19h13M3 5h.1M3 12h.1M3 19h.1"/>',
  pause: '<path d="M8 5v14M16 5v14" stroke-width="5"/>',
  restart: '<path d="M5 5v14m14-14L7 12l12 7Z" fill="currentColor"/>',
  next: '<path d="M19 5v14M5 5l12 7-12 7Z" fill="currentColor"/>',
  more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  up: '<path d="m6 15 6-6 6 6"/>',
  down: '<path d="m6 9 6 6 6-6"/>',
  add: '<path d="M12 5v14M5 12h14"/>',
  remove: '<path d="M5 12h14"/>',
  search: '<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>',
  download: '<path d="M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5"/>',
  home: '<path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z"/>',
  movies:
    '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M7 4v16M17 4v16M3 9h4M3 15h4M17 9h4M17 15h4"/>',
  shows:
    '<rect x="3" y="6" width="18" height="13" rx="3"/><path d="m8 2 4 4 4-4M8 22h8"/>',
  music:
    '<path d="M9 18V5l11-2v13M9 9l11-2"/><ellipse cx="6" cy="18" rx="3" ry="3"/><ellipse cx="17" cy="16" rx="3" ry="3"/>',
  books:
    '<path d="M12 5C9 3 5 3 2 4v15c3-1 7-1 10 1 3-2 7-2 10-1V4c-3-1-7-1-10 1Zm0 0v15"/>',
} as const;
export function NavigationIcon({
  name,
  color,
  size = 24,
}: {
  name: keyof typeof paths;
  color: string;
  size?: number;
}) {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" color="${color}" stroke="${color}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${paths[name]}</svg>`;
  return (
    <Image
      accessible={false}
      aria-hidden
      source={{ uri: svgDataURI(svg) }}
      style={{ width: size, height: size }}
    />
  );
}
