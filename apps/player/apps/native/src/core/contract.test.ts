import {
  parseChallenge,
  parseItem,
  parseLibrary,
  parseMe,
  parseMediaItem,
  parsePending,
  parsePlayback,
  parseProgress,
  parseQuickConnectToken,
  type InputValue,
} from './contract';

const invalid = 'Kinosail Server returned an invalid response.';
const minimal = { id: 'arrival', kind: 'video', title: 'Arrival' };

describe('native response contract', () => {
  it.each([null, undefined, false, true, 0, 1, '', 'not a record', []])(
    'rejects a non-object response %#',
    (input) => {
      expect(() => parseProgress(input)).toThrow(invalid);
    },
  );

  it.each(['false', 'true', 0, 1, null, {}, []])(
    'rejects a non-boolean watched value %#',
    (watched) => {
      expect(() => parseProgress({ watched })).toThrow(invalid);
    },
  );

  it.each([false, true])('preserves watched=%s without coercion', (watched) => {
    expect(
      parseProgress({
        watched,
        seconds: 12.5,
        revision: 3,
        session: 'native-viewer',
      }),
    ).toEqual({
      watched,
      seconds: 12.5,
      revision: 3,
      session: 'native-viewer',
    });
  });

  it('defaults omitted progress fields', () => {
    expect(parseProgress({})).toEqual({
      seconds: 0,
      watched: false,
      revision: 0,
      session: '',
    });
  });

  it('accepts explicitly zero progress', () => {
    expect(parseProgress({ seconds: 0, revision: 0 })).toEqual(
      parseProgress({}),
    );
  });

  it.each([-1, NaN, Infinity, '1', null, false])(
    'rejects invalid elapsed seconds %#',
    (seconds) => {
      expect(() => parseProgress({ seconds })).toThrow(invalid);
    },
  );

  it.each([-1, 1.5, Number.MAX_SAFE_INTEGER + 1, NaN, '2'])(
    'rejects invalid revision %#',
    (revision) => {
      expect(() => parseProgress({ revision })).toThrow(invalid);
    },
  );

  it.each([
    { code: '12345', secret: 'secret' },
    { code: 'abcdef', secret: 'secret' },
    { code: '12345a', secret: 'secret' },
    { code: 'a12345', secret: 'secret' },
    { code: '', secret: 'secret' },
    { code: '1234567', secret: 'secret' },
    { code: '123456', secret: '' },
    { code: '123456', secret: 'x'.repeat(513) },
  ])('rejects an invalid challenge %#', (input) => {
    expect(() => parseChallenge(input)).toThrow(invalid);
  });

  it('parses valid authorization responses', () => {
    expect(parseChallenge({ code: '000001', secret: 'secret' })).toEqual({
      code: '000001',
      secret: 'secret',
    });
    expect(parseQuickConnectToken({ token: 'viewer-token' })).toBe(
      'viewer-token',
    );
    expect(parsePending({ status: 'pending' })).toBeUndefined();
    expect(() => parsePending({ status: 'approved' })).toThrow(invalid);
    expect(() => parseQuickConnectToken({ token: '' })).toThrow(invalid);
  });

  it('requires a boolean viewer owner flag', () => {
    const me = {
      server: 'Den',
      viewer: { id: 'viewer', name: 'Mike', owner: false },
    };
    expect(parseMe(me)).toEqual(me);
    expect(() =>
      parseMe({ ...me, viewer: { ...me.viewer, owner: 'false' } }),
    ).toThrow(invalid);
  });

  it.each(['server', 'id', 'name'])('requires nonempty viewer %s', (field) => {
    const me = {
      server: 'Den',
      viewer: { id: 'viewer', name: 'Mike', owner: false },
    };
    const value =
      field === 'server'
        ? { ...me, server: '' }
        : { ...me, viewer: { ...me.viewer, [field]: '' } };
    expect(() => parseMe(value)).toThrow(invalid);
  });

  it('parses a minimal item with empty optional metadata', () => {
    const parsed = parseMediaItem(minimal);
    expect(parsed).toMatchObject({
      ...minimal,
      plot: '',
      season: 0,
      episode: 0,
      artwork: '',
      backdrop: '',
      progress: { seconds: 0, watched: false, session: '', revision: 0 },
    });
    expect(parseItem({ item: minimal })).toEqual(parsed);
    expect(parseLibrary({ items: [minimal] })).toEqual([parsed]);
  });

  it.each(['id', 'kind', 'title'])('requires media %s', (field) => {
    expect(() => parseMediaItem({ ...minimal, [field]: '' })).toThrow(invalid);
  });

  it.each([
    ['id', 128],
    ['kind', 32],
    ['title', 512],
    ['year', 16],
    ['plot', 10_000],
    ['rating', 32],
    ['tagline', 512],
    ['genres', 512],
    ['director', 256],
    ['studio', 256],
    ['artist', 256],
    ['album', 256],
    ['show', 256],
    ['artwork', 2048],
    ['backdrop', 2048],
    ['container', 32],
  ] as const)('enforces the %s text limit', (field, maximum) => {
    const value = 'x'.repeat(maximum);
    expect(parseMediaItem({ ...minimal, [field]: value })[field]).toBe(value);
    expect(() => parseMediaItem({ ...minimal, [field]: `${value}x` })).toThrow(
      invalid,
    );
    expect(() => parseMediaItem({ ...minimal, [field]: 42 })).toThrow(invalid);
    if (!['id', 'kind', 'title'].includes(field))
      expect(parseMediaItem({ ...minimal, [field]: '' })[field]).toBe('');
  });

  it.each([undefined, null, {}, Array(201).fill(minimal)] as InputValue[])(
    'rejects an invalid library collection %#',
    (items) => {
      expect(() => parseLibrary({ items })).toThrow(invalid);
    },
  );

  it('accepts exactly the library size limit', () => {
    expect(parseLibrary({ items: Array(200).fill(minimal) })).toHaveLength(200);
  });

  it.each(['mode', 'reason'])('requires playback plan %s', (field) => {
    const plan = {
      allowed: true,
      mode: 'direct',
      reason: 'compatible',
      [field]: '',
    };
    expect(() => parsePlayback({ plan })).toThrow(invalid);
  });

  it('requires an explicit boolean playback permission', () => {
    const plan = {
      allowed: false,
      mode: 'denied',
      reason: 'incompatible',
    };
    expect(parsePlayback({ plan })).toEqual({
      directAllowed: false,
      details: { chapters: [], download: '', next: '' },
      compatible: undefined,
      direct: '',
      directType: '',
      duration: 0,
      start: 0,
      progressToken: '',
      plan,
    });
    expect(() =>
      parsePlayback({ plan: { ...plan, allowed: 'false' } }),
    ).toThrow(invalid);
  });
});
