import React, { useEffect, useState } from 'react';
import { readerResource } from '@/core/reader';
import type { ReaderContentProps } from './reader-content.types';
export function ReaderContent({
  server,
  authorization,
  id,
  path,
  type,
  fontSize,
  theme,
  onError,
}: ReaderContentProps) {
  const [content, setContent] = useState('');
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    const read = async (resource: string) => {
      readerResource(resource, id);
      const response = await fetch(server + resource, {
        headers: { Authorization: authorization },
        credentials: 'omit',
        redirect: 'error',
        cache: 'no-store',
        signal: controller.signal,
      });
      if (
        !response.ok ||
        Number(response.headers.get('content-length')) > 32 * 1024 ** 2 ||
        !response.body
      )
        throw new Error('Book unavailable.');
      const reader = response.body.getReader(),
        chunks: Uint8Array[] = [];
      let size = 0;
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          size += value.length;
          if (size > 32 * 1024 ** 2) throw new Error('Book too large.');
          chunks.push(value);
        }
      } finally {
        await reader.cancel();
      }
      return new Blob(chunks as BlobPart[], {
        type:
          response.headers.get('content-type') || 'application/octet-stream',
      });
    };
    const dataURL = (blob: Blob) =>
      new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result));
        reader.onerror = reject;
        reader.readAsDataURL(blob);
      });
    const load = async () => {
      const blob = await read(path);
      if (type === 'pdf') {
        const url = await dataURL(blob);
        if (active)
          setContent(
            `<embed src="${url}" type="application/pdf" style="width:100%;height:100vh">`,
          );
        return;
      }
      let body = '';
      if (type === 'comic')
        body = `<img alt="Book page" src="${await dataURL(blob)}">`;
      else {
        const doc = new DOMParser().parseFromString(
          await blob.text(),
          'text/html',
        );
        doc
          .querySelectorAll(
            'script,iframe,object,embed,link,base,form,meta,style,svg,math',
          )
          .forEach((node) => node.remove());
        const images = Array.from(doc.querySelectorAll('img'));
        if (images.length > 64) throw new Error('Too many book images.');
        for (const image of images) {
          const original = image.getAttribute('src') || '';
          image.removeAttribute('srcset');
          try {
            const resource = new URL(original, server + path);
            if (resource.origin !== server) throw new Error();
            image.src = await dataURL(await read(resource.pathname));
          } catch {
            image.removeAttribute('src');
          }
        }
        doc.querySelectorAll('*').forEach((node) => {
          for (const attr of Array.from(node.attributes)) {
            if (
              attr.name.toLowerCase().startsWith('on') ||
              ['style', 'href', 'action', 'formaction', 'background'].includes(
                attr.name.toLowerCase(),
              )
            )
              node.removeAttribute(attr.name);
          }
        });
        body = doc.body.innerHTML;
      }
      const colors =
        theme === 'dark'
          ? ['#151719', '#eceae5']
          : theme === 'sepia'
            ? ['#f2e7cc', '#342b20']
            : ['#fff', '#202020'];
      if (active)
        setContent(
          `<meta http-equiv="Content-Security-Policy" content="default-src 'none';img-src data:;style-src 'unsafe-inline';base-uri 'none';form-action 'none'"><style>body{padding:20px;background:${colors[0]};color:${colors[1]};font-size:${fontSize}px;line-height:1.6}img{max-width:100%;height:auto}</style>${body}`,
        );
    };
    void load().catch(() => {
      if (active) onError();
    });
    return () => {
      active = false;
      controller.abort();
    };
  }, [server, authorization, id, path, type, fontSize, theme]);
  return (
    <iframe
      title="Book reader"
      sandbox=""
      srcDoc={content}
      style={{ border: 0, flex: 1, width: '100%', minHeight: 240 }}
    />
  );
}
