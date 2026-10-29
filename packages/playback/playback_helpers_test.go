package playback

import (
	"context"
	"os/exec"
)

func RunHLSCommand(command *exec.Cmd, private ...string) error {
	return RunHLSCommandObserved(command, nil, private...)
}

func AutomaticSkipAudioArguments(recipe HLSRecipe, policy HLSRecipePolicy) []string {
	filter := automaticSkipAudioFilter(recipe, policy)
	if filter == "" {
		return nil
	}
	return []string{"-af", filter}
}

func PublishVariants(ctx context.Context, source, directory, transcoder string, qualities []PlaybackQuality, results <-chan error, expected int, independent bool, write AtomicWriter) error {
	return PublishVariantsObserved(ctx, source, directory, transcoder, qualities, results, expected, independent, write, nil)
}

func WaitHLSReady(ctx context.Context, probe func() error) bool {
	return WaitHLSChange(ctx, nil, nil, probe)
}
