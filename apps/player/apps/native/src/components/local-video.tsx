import React from 'react';
import type { LocalVideoProps, VLCPlayerRef } from './local-video.types';
export type { VLCPlayerRef, VLCPlayerTracks } from './local-video.types';
export default React.forwardRef<VLCPlayerRef, LocalVideoProps>(
  function LocalVideo() {
    return null;
  },
);
