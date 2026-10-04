// Disposable host fixture: real app binaries, synthetic media, no containers.
import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
const [app, port] = process.argv.slice(2);
if (!['player', 'subtitles'].includes(app) || !/^\d{1,5}$/.test(port) || +port < 1024 || +port > 65535) throw new Error('invalid fixture app/port');
const root = mkdtempSync(join(tmpdir(), `kinosail-e2e-${app}-`));
for (const dir of ['media/Movies', 'data', 'cache', 'backups']) mkdirSync(join(root, dir), { recursive: true });
const media = join(root, 'media/Movies');
const generated = spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-f', 'lavfi', '-i', 'testsrc2=size=320x180:rate=24', '-f', 'lavfi', '-i', 'sine=frequency=220:sample_rate=48000', '-t', '8', '-c:v', 'libx264', '-preset', 'ultrafast', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-movflags', '+faststart', join(media, 'Example Movie.mp4')], { stdio: 'inherit' });
if (generated.status !== 0) { rmSync(root, { recursive: true, force: true }); throw new Error('FFmpeg fixture failed'); }
writeFileSync(join(media, 'Example Movie.en.srt'), '1\n00:00:00,000 --> 00:00:03,000\nExample dialogue.\n\n2\n00:00:03,500 --> 00:00:07,000\nA second line.\n');
const binary = process.env[`KINOSAIL_E2E_${app.toUpperCase()}_BINARY`] ?? join(process.cwd(), '.e2e/bin', app);
const child = spawn(binary, [], { stdio: 'inherit', env: { ...process.env, KINOSAIL_LISTEN: `127.0.0.1:${port}`, KINOSAIL_TLS_ENABLED: 'false', KINOSAIL_MEDIA_DIR: join(root, 'media'), KINOSAIL_DATA_DIR: join(root, 'data'), KINOSAIL_CACHE_DIR: join(root, 'cache'), KINOSAIL_BACKUP_DIR: join(root, 'backups'), KINOSAIL_LIBRARIES: '["Movies"]', KINOSAIL_SCAN_INTERVAL: '24h', KINOSAIL_BACKUP_INTERVAL: '24h', KINOSAIL_SERVER_NAME: 'Kinosail E2E Fixture' } });
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal));
child.on('error', error => { rmSync(root, { recursive: true, force: true }); throw error; });
child.on('exit', code => { rmSync(root, { recursive: true, force: true }); process.exitCode = code ?? 0; });
