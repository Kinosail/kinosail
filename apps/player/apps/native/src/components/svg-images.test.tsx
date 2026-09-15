import React from 'react';
import { render } from '@testing-library/react-native';
import { Image } from 'expo-image';
import qrcode from 'qrcode-generator';
import { ApprovalQR } from './approval-qr';
import { NavigationIcon } from './navigation-icon';

jest.mock('expo-image', () => ({ Image: jest.fn(() => null) }));

function svgSource() {
  const props = jest.mocked(Image).mock.calls.at(-1)?.[0];
  const source = props?.source as { uri: string };
  expect(source.uri).toMatch(/^data:image\/svg\+xml;base64,[A-Za-z0-9+/]+=*$/);
  return Buffer.from(source.uri.split(',')[1], 'base64').toString('utf8');
}

it('provides Android-decodable navigation icons with the requested color and size', async () => {
  await render(<NavigationIcon name="home" color="#c8f169" size={28} />);
  const svg = svgSource();
  expect(svg).toContain('stroke="#c8f169"');
  expect(svg).toContain('<path');
  expect(jest.mocked(Image).mock.calls.at(-1)?.[0]).toMatchObject({
    accessible: false,
    style: { width: 28, height: 28 },
  });
});

it('provides the complete local approval QR as Android-decodable image data', async () => {
  await render(<ApprovalQR server="https://kino.example" code="123456" />);
  const expected = qrcode(0, 'M');
  expected.addData('https://kino.example/connect?code=123456');
  expected.make();
  expect(svgSource()).toBe(expected.createSvgTag({ cellSize: 4, margin: 16 }));
  expect(jest.mocked(Image).mock.calls.at(-1)?.[0]).toMatchObject({
    cachePolicy: 'none',
    accessibilityRole: 'image',
  });
});
