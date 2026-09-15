import { encode } from 'js-base64';

// Expo Image on Android decodes data URLs as base64.
export const svgDataURI = (svg: string) =>
  `data:image/svg+xml;base64,${encode(svg)}`;
