#import "KinosailAudioProcessor.h"
#import <VLCKit/VLCKit.h>
#import <VLCKit/vlc/libvlc.h>
#import <VLCKit/vlc/libvlc_media_player.h>
#import <AudioToolbox/AudioToolbox.h>
#import <mach/mach_time.h>
#import <math.h>
#import <objc/runtime.h>

// VLCKit 4.0.0a24 exposes this accessor in VLCMediaPlayer+Internal.h. Keep the
// pinned framework's player, events, and video clock; replace only audio output.
@interface VLCMediaPlayer (KinosailAudioInstance)
@property (readonly) libvlc_media_player_t *playerInstance;
@end

static const unsigned BufferCount = 24;
static const unsigned BufferFrames = 8192;

@interface KinosailAudioProcessor () {
 AudioQueueRef _queue;
 NSCondition *_condition;
 dispatch_queue_t _output;
 NSUInteger _epoch;
 BOOL _starting;
 NSMutableArray<NSValue *> *_free;
 BOOL _failed, _closed, _started, _night, _dialogue, _resetting;
 double _boost, _envelope, _limiter, _low[2], _high[2];
}
- (void)enqueue:(const float *)samples count:(unsigned)count pts:(int64_t)pts;
- (void)pause;
- (void)resume;
- (void)flush;
- (void)drain;
- (void)returnBuffer:(AudioQueueBufferRef)buffer;
- (void)failLocked;
@end

static void returned(void *data, AudioQueueRef queue, AudioQueueBufferRef buffer) {
 [(__bridge KinosailAudioProcessor *)data returnBuffer:buffer];
}
static void playAudio(void *data, const void *samples, unsigned count, int64_t pts) {
 [(__bridge KinosailAudioProcessor *)data enqueue:samples count:count pts:pts];
}
static void pauseAudio(void *data, int64_t pts) { [(__bridge KinosailAudioProcessor *)data pause]; }
static void resumeAudio(void *data, int64_t pts) { [(__bridge KinosailAudioProcessor *)data resume]; }
static void flushAudio(void *data, int64_t pts) { [(__bridge KinosailAudioProcessor *)data flush]; }

static void drainAudio(void *data) { [(__bridge KinosailAudioProcessor *)data drain]; }

@implementation KinosailAudioProcessor
- (instancetype)initWithPlayer:(VLCMediaPlayer *)player nightMode:(BOOL)night dialogueBoost:(BOOL)dialogue volumeBoost:(double)boost {
 if (!isfinite(boost) || boost < 1 || boost > 2 || ![player respondsToSelector:@selector(playerInstance)]) return nil;
 if (!(self = [super init])) return nil;
 _condition = [NSCondition new]; _free = [NSMutableArray new];
 _output = dispatch_queue_create("com.kinosail.player.audio-output", DISPATCH_QUEUE_SERIAL);
 _night = night; _dialogue = dialogue; _boost = boost; _limiter = 1;
 AudioStreamBasicDescription format = { .mSampleRate = 48000, .mFormatID = kAudioFormatLinearPCM,
  .mFormatFlags = kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked, .mBytesPerPacket = 8,
  .mFramesPerPacket = 1, .mBytesPerFrame = 8, .mChannelsPerFrame = 2, .mBitsPerChannel = 32 };
 if (AudioQueueNewOutput(&format, returned, (__bridge void *)self, NULL, NULL, 0, &_queue) != noErr) return nil;
 for (unsigned i = 0; i < BufferCount; i++) {
  AudioQueueBufferRef buffer = NULL;
  if (AudioQueueAllocateBuffer(_queue, BufferFrames * 8, &buffer) != noErr) { [self close]; return nil; }
  [_free addObject:[NSValue valueWithPointer:buffer]];
 }
 objc_setAssociatedObject(player, @selector(playerInstance), self, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
 libvlc_audio_set_format(player.playerInstance, "FL32", 48000, 2);
 libvlc_audio_set_callbacks(player.playerInstance, playAudio, pauseAudio, resumeAudio, flushAudio, drainAudio, (__bridge void *)self);
 return self;
}
- (void)returnBuffer:(AudioQueueBufferRef)buffer {
 [_condition lock];
 if (!_closed && ![_free containsObject:[NSValue valueWithPointer:buffer]]) [_free addObject:[NSValue valueWithPointer:buffer]];
 [_condition signal]; [_condition unlock];
}
- (void)enqueue:(const float *)samples count:(unsigned)count pts:(int64_t)pts {
 if (!samples || count == 0 || count > 192000) return;
 unsigned offset = 0;
 while (offset < count) {
  [_condition lock];
  while (!_closed && !_failed && (_resetting || _free.count == 0)) [_condition wait];
  if (_closed || _failed) { [_condition unlock]; return; }
  AudioQueueBufferRef buffer = _free.lastObject.pointerValue; [_free removeLastObject];
  unsigned frames = MIN(BufferFrames, count - offset);
  float *output = buffer->mAudioData;
  for (unsigned i = 0; i < frames; i++) {
   double pair[2];
   for (unsigned channel = 0; channel < 2; channel++) {
    double sample = samples[(offset + i) * 2 + channel];
    if (!isfinite(sample)) sample = 0;
    sample = fmax(-8, fmin(8, sample));
    // Speech emphasis, not voice separation: a broad 200–3000 Hz band.
    _low[channel] += .02585 * (sample - _low[channel]);
    _high[channel] += .32477 * (sample - _high[channel]);
    pair[channel] = (sample + (_dialogue ? .8 * (_high[channel] - _low[channel]) : 0)) * _boost;
   }
   double peak = fmax(fabs(pair[0]), fabs(pair[1]));
   _envelope += (peak > _envelope ? .004158 : .000139) * (peak - _envelope);
   double compression = _night && _envelope > .12 ? pow(.12 / _envelope, .75) : 1;
   double gain = compression * (_night ? 1.8 : 1);
   double wanted = peak * gain > .97 ? .97 / (peak * gain) : 1;
   _limiter = wanted < _limiter ? wanted : _limiter + .000208 * (wanted - _limiter);
   for (unsigned channel = 0; channel < 2; channel++) output[i * 2 + channel] = (float)fmax(-.97, fmin(.97, pair[channel] * gain * _limiter));
  }
  buffer->mAudioDataByteSize = frames * 8;
  NSUInteger epoch = _epoch;
  [_condition unlock];
  dispatch_async(_output, ^{
   [self->_condition lock];
   if (self->_closed || self->_failed || epoch != self->_epoch) {
    if (!self->_closed) [self->_free addObject:[NSValue valueWithPointer:buffer]];
    [self->_condition broadcast]; [self->_condition unlock]; return;
   }
   AudioQueueRef queue = self->_queue;
   BOOL start = !self->_started; self->_started = YES; self->_starting = start;
   [self->_condition unlock];
   if (start) dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC), dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
    [self->_condition lock];
    if (!self->_closed && self->_starting && epoch == self->_epoch) [self failLocked];
    [self->_condition unlock];
   });
   OSStatus status = AudioQueueEnqueueBuffer(queue, buffer, 0, NULL);
   if (status == noErr && start) {
    // Start against libVLC's clock. Never hold the buffer lock across system IO.
    int64_t delay = pts - libvlc_clock();
    mach_timebase_info_data_t timebase; mach_timebase_info(&timebase);
    uint64_t ticks = (uint64_t)(fmax(0, fmin(2000000, delay)) * 1000.0 * timebase.denom / timebase.numer);
    AudioTimeStamp time = { .mHostTime = mach_absolute_time() + ticks, .mFlags = kAudioTimeStampHostTimeValid };
    status = AudioQueueStart(queue, &time);
   }
   [self->_condition lock]; self->_starting = NO;
   if (status != noErr) [self failLocked];
   [self->_condition unlock];
  });
  offset += frames;
 }
}
- (void)failLocked {
 if (_failed) return; _failed = YES; [_condition broadcast];
 dispatch_async(dispatch_get_main_queue(), ^{ if (self.onFailure) self.onFailure(); });
}
- (void)pause {
 dispatch_async(_output, ^{
  [self->_condition lock]; BOOL available = !self->_closed && !self->_failed; AudioQueueRef queue = self->_queue; [self->_condition unlock];
  if (available && AudioQueuePause(queue) != noErr) { [self->_condition lock]; [self failLocked]; [self->_condition unlock]; }
 });
}
- (void)resume {
 dispatch_async(_output, ^{
  [self->_condition lock]; BOOL available = !self->_closed && !self->_failed && self->_started; AudioQueueRef queue = self->_queue; [self->_condition unlock];
  if (available && AudioQueueStart(queue, NULL) != noErr) { [self->_condition lock]; [self failLocked]; [self->_condition unlock]; }
 });
}
- (void)flush {
 [_condition lock]; if (_closed || _failed) { [_condition unlock]; return; }
 _epoch++; _resetting = YES; _started = NO; _starting = NO;
 _envelope = 0; _limiter = 1; _low[0] = _low[1] = _high[0] = _high[1] = 0;
 [_condition unlock];
 dispatch_async(_output, ^{
  [self->_condition lock]; BOOL available = !self->_closed; AudioQueueRef queue = self->_queue; [self->_condition unlock];
  OSStatus status = available ? AudioQueueReset(queue) : noErr;
  [self->_condition lock]; if (status != noErr) [self failLocked]; self->_resetting = NO; [self->_condition broadcast]; [self->_condition unlock];
 });
}
- (void)drain {
 NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:10];
 [_condition lock];
 while (!_closed && !_failed && _free.count < BufferCount) {
  if (![_condition waitUntilDate:deadline]) { [self failLocked]; break; }
 }
 [_condition unlock];
}
- (void)close {
 [_condition lock]; if (_closed) { [_condition unlock]; return; }
 _closed = YES; [_condition broadcast]; [_condition unlock];
 // Decoding and UI teardown must not wait for a stalled hardware audio route.
 // This block also retains the callback context until all output work finishes.
 dispatch_async(_output, ^{
  AudioQueueRef queue = self->_queue; self->_queue = NULL;
  if (queue) AudioQueueDispose(queue, true);
 });
}
- (void)dealloc { if (_queue) AudioQueueDispose(_queue, true); }
@end
