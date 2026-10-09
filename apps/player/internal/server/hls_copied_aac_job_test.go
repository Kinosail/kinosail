package server

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
)

// Gap: a public request cannot hold an old worker at its final join boundary.
func TestCopiedAACReplacementCancelsAndJoinsBeforeCacheMutation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	copiedAACSourceRoots(manager, item)
	copiedRecoveryProbe(t, manager, "")
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
		t.Fatal(err)
	}
	before := copiedRecoveryPreserved(t, directory)
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	key := hlsRecipeKey(item.ID, recipe)
	if err := manager.recordCopiedAACPolicy(key, policy, info, true, 1); err != nil {
		t.Fatal(err)
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	oldContext, old := manager.newHLSJob(t.Context(), 0)
	old.cachePolicy = policy
	manager.jobs[key] = old
	defer func() {
		old.cancel(context.Canceled)
		manager.mu.Lock()
		if manager.jobs[key] == old {
			delete(manager.jobs, key)
		}
		manager.mu.Unlock()
		close(old.done)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err = manager.ensureHLSJob(ctx, item, key, options, recipe)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(context.Cause(oldContext), errHLSIdentityChanged) {
		t.Fatal("old policy worker was adopted or replacement skipped its cancellation/join")
	}
	if manager.jobs[key] != old {
		t.Fatal("replacement changed ownership before old worker joined")
	}
	before()
}

func TestCopiedAACRefillKeepsCanonicalPolicy(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, item, recipe, _, base, _ := copiedRecoveryFixture(t)
	copiedAACSourceRoots(manager, item)
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	key := hlsRecipeKey(item.ID, recipe)
	if err := manager.recordCopiedAACPolicy(key, base, info, true, 1); err != nil {
		t.Fatal(err)
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	worker := recipe
	worker.offset = 20
	ctx := context.WithValue(t.Context(), copiedAACWorkerKey{}, &copiedAACWorkerIdentity{key: key, recipe: recipe, policy: options.Cache})
	if err := manager.validateHLSPolicy(ctx, item, worker, options.Cache); err != nil {
		t.Fatal("refill offset lost the canonical producer policy")
	}
	if err := manager.validateHLSPolicy(t.Context(), item, worker, options.Cache); err == nil {
		t.Fatal("the different recipe accidentally shared canonical qualification")
	}
}

func TestCopiedAACLiveIndexOwnerCannotBeReplacedByLookalike(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified retained-source producer is Linux-only")
	}
	manager, item, recipe, _, base, _ := copiedRecoveryFixture(t)
	copiedAACSourceRoots(manager, item)
	info, err := os.Stat(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	key := hlsRecipeKey(item.ID, recipe)
	if err := manager.recordCopiedAACPolicy(key, base, info, true, 1); err != nil {
		t.Fatal(err)
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal(err)
	}
	ctx, job := manager.newHLSJob(t.Context(), 0)
	defer job.cancel(context.Canceled)
	job.cachePolicy = options.Cache
	manager.jobs[key] = job
	ctx = context.WithValue(ctx, copiedAACWorkerKey{}, &copiedAACWorkerIdentity{key: key, recipe: recipe, policy: options.Cache, job: job})
	if !manager.copiedAACWorkerCurrent(ctx, key, options.Cache) {
		t.Fatal("live observed owner was rejected")
	}
	manager.jobs["unrelated"] = &hlsJob{}
	if manager.copiedAACWorkerCurrent(ctx, key, options.Cache) {
		t.Fatal("unrelated live job widened idle index admission")
	}
	delete(manager.jobs, "unrelated")
	lookalike := *job
	manager.jobs[key] = &lookalike
	if manager.copiedAACWorkerCurrent(ctx, key, options.Cache) {
		t.Fatal("lookalike pointer acquired the old index publication")
	}
	manager.jobs[key] = job
	if manager.copiedAACWorkerCurrent(ctx, key, base) {
		t.Fatal("legacy policy acquired owned reindex admission")
	}
	job.cancel(context.Canceled)
	if manager.copiedAACWorkerCurrent(ctx, key, options.Cache) {
		t.Fatal("cancelled owner retained final publication admission")
	}
	delete(manager.jobs, key)
}
