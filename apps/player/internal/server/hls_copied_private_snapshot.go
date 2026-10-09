package server

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
)

func (assets *copiedHLSPrivateAssets) audio(ctx context.Context) (*copiedHLSPrivateAudioFacts, error) {
	if !assets.bound(ctx) {
		return nil, errCopiedHLSIndex
	}
	var snapshots [2][]byte
	for number, file := range assets.files {
		data, err := copiedHLSPrivateFileSnapshot(ctx, file, assets.info[number].Size())
		if err != nil {
			return nil, err
		}
		snapshots[number] = data
	}
	facts, err := parseCopiedHLSPrivateAudio(ctx, snapshots[0], snapshots[1])
	if err != nil || !assets.current(ctx, facts) {
		return nil, errCopiedHLSIndex
	}
	return facts, nil
}

// Hash the same held files again; inode, size and mtime cannot detect an
// equal-length rewrite whose mtime was restored. This remains an observed
// snapshot check, not an immutable-content or producing-worker certificate.
func (assets *copiedHLSPrivateAssets) current(ctx context.Context, facts *copiedHLSPrivateAudioFacts) bool {
	if facts == nil || !assets.bound(ctx) {
		return false
	}
	expected := [2][32]byte{facts.Initialization, facts.First}
	for number, file := range assets.files {
		hash, err := copiedHLSPrivateFileHash(ctx, file, assets.info[number].Size())
		if err != nil || hash != expected[number] {
			return false
		}
	}
	return assets.bound(ctx)
}

func copiedHLSPrivateFileSnapshot(ctx context.Context, file *os.File, size int64) ([]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errCopiedHLSIndex
	}
	data, err := io.ReadAll(copiedHLSContextReader{ctx, io.LimitReader(file, size+1)})
	if err != nil || ctx.Err() != nil || int64(len(data)) != size {
		return nil, errCopiedHLSIndex
	}
	return data, nil
}

func copiedHLSPrivateFileHash(ctx context.Context, file *os.File, size int64) ([32]byte, error) {
	var result [32]byte
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return result, errCopiedHLSIndex
	}
	hash := sha256.New()
	count, err := io.Copy(hash, copiedHLSContextReader{ctx, io.LimitReader(file, size+1)})
	if err != nil || ctx.Err() != nil || count != size {
		return result, errCopiedHLSIndex
	}
	copy(result[:], hash.Sum(nil))
	return result, nil
}
