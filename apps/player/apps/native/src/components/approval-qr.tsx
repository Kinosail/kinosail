import { Image } from 'expo-image';
import qrcode from 'qrcode-generator';
import React, { useMemo } from 'react';
import { View } from 'react-native';
import { approvalURL } from '@/core/approval-link';
import { svgDataURI } from '@/core/svg-data-uri';

export function ApprovalQR({
  server,
  code,
  size = 248,
}: {
  server: string;
  code: string;
  size?: number;
}) {
  const uri = useMemo(() => {
    const qr = qrcode(0, 'M');
    qr.addData(approvalURL(server, code));
    qr.make();
    // Four quiet modules on every edge; no external QR service or session secret.
    return svgDataURI(qr.createSvgTag({ cellSize: 4, margin: 16 }));
  }, [server, code]);
  return (
    <View style={{ alignItems: 'center' }}>
      <Image
        accessibilityLabel="Scan with your phone camera to approve this device"
        accessibilityRole="image"
        source={{ uri }}
        style={{ width: size, height: size, maxWidth: '100%', aspectRatio: 1 }}
        contentFit="contain"
        cachePolicy="none"
      />
    </View>
  );
}
