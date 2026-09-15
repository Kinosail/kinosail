#import <Foundation/Foundation.h>
@class VLCMediaPlayer;

NS_ASSUME_NONNULL_BEGIN
// Opt-in decoded stereo processing. The ordinary VLC/AVKit output path remains
// in charge when no audio effect is selected.
@interface KinosailAudioProcessor : NSObject
- (nullable instancetype)initWithPlayer:(VLCMediaPlayer *)player nightMode:(BOOL)night dialogueBoost:(BOOL)dialogue volumeBoost:(double)boost;
@property (nonatomic, copy, nullable) void (^onFailure)(void);
- (void)close;
@end
NS_ASSUME_NONNULL_END
