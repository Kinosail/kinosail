import type { Progress } from './contract';
import { createProgressWriter } from './progress-writer';
const saved = (event: Partial<Progress>): Progress => ({
  seconds: 0,
  watched: false,
  session: 'session',
  revision: 1,
  ...event,
});
describe('progress writer', () => {
  it('sends strictly newer revisions and serializes coalesced saves', async () => {
    let resolve!: (value: Progress) => void;
    const first = new Promise<Progress>((done) => {
      resolve = done;
    });
    const save = jest
      .fn()
      .mockReturnValueOnce(first)
      .mockImplementation(async (event) => saved(event));
    const writer = createProgressWriter(
      { session: 'session', revision: 7, playbackToken: 'timeline' },
      save,
    );
    writer.write(10);
    writer.write(11);
    writer.write(12);
    expect(save).toHaveBeenCalledTimes(1);
    resolve(saved({ revision: 8 }));
    await writer.drain();
    expect(
      save.mock.calls.map(([event]) => [
        event.seconds,
        event.revision,
        event.watched,
        event.playbackToken,
      ]),
    ).toEqual([
      [10, 8, false, 'timeline'],
      [12, 9, false, 'timeline'],
    ]);
  });
  it('retains a failed final save for retry without spinning', async () => {
    const save = jest
      .fn()
      .mockRejectedValueOnce(new Error('offline'))
      .mockImplementation(async (event) => saved(event));
    const writer = createProgressWriter(
      { session: 'session', revision: 0 },
      save,
    );
    writer.write(32);
    await writer.drain();
    expect(save).toHaveBeenCalledTimes(1);
    writer.retry();
    await writer.drain();
    expect(save.mock.calls[1][0]).toMatchObject({ seconds: 32, revision: 2 });
  });
  it('preserves completion through teardown and explicitly clears it for replay', async () => {
    const save = jest.fn(async (event) => saved(event));
    const writer = createProgressWriter(
      { session: 'session', revision: 0 },
      save,
    );
    writer.write(100, true);
    await writer.drain();
    writer.write(100);
    await writer.drain();
    writer.write(0, false);
    await writer.drain();
    writer.write(5);
    await writer.drain();
    expect(save.mock.calls.map(([event]) => event.watched)).toEqual([
      true,
      true,
      false,
      false,
    ]);
  });
  it.each([NaN, Infinity, -1, 31536001])(
    'rejects invalid position %s without saving',
    async (seconds) => {
      const save = jest.fn();
      const writer = createProgressWriter(
        { session: 'session', revision: 0 },
        save,
      );
      writer.write(seconds);
      await writer.drain();
      expect(save).not.toHaveBeenCalled();
    },
  );
});
