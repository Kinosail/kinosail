package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/privatefile"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

const hlsSourcePolicyLimit = 16 * 1024

var ErrHLSSourceChanged = errors.New("HLS source snapshot changed")

// BindHLSEncoder keeps seek segments on the encoder that produced the existing
// initialization data, while cache readers use the unchanged playback policy.
func BindHLSEncoder(directory string, options transcodepolicy.Settings, resume bool) error {
	identity := struct {
		Accelerator, Encoder, Device, Codec, HDR string
		ToneMap                                  string `json:",omitempty"`
	}{options.Accelerator, options.Encoder, options.Device, options.Codec, options.OutputHDR, options.HardwareToneMap}
	data, err := json.Marshal(identity)
	if err != nil || len(data) > 1024 || identity.Encoder == "" {
		return errors.New("HLS encoder identity is invalid")
	}
	path := filepath.Join(directory, ".encoder")
	if resume {
		previous, err := privatefile.Read(path, 1024)
		if err != nil || !bytes.Equal(data, previous) {
			return errors.New("playback encoder changed; start a new compatible stream")
		}
		return nil
	}
	return privatefile.WriteCache(path, data)
}

// BindHLSSource records the immutable source and complete policy before encoding.
// A retained presentation is never rebound to a different source or policy.
func BindHLSSource(directory, source, policy string) error {
	if err := ValidateHLSSource(source, policy); err != nil {
		return err
	}
	root, err := openHLSSourceRoot(directory)
	if err != nil {
		return errors.New("HLS source cache cannot be opened")
	}
	defer root.Close()
	previous, err := readHLSSource(root)
	if err == nil {
		if string(previous) == policy {
			return nil
		}
		return errors.New("HLS source cache identity changed")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.New("HLS source cache identity is invalid")
	}
	if !hlsSourceBindable(root, directory, source, policy) {
		return errors.New("HLS source cache has unvalidated output")
	}
	if err := writeHLSSource(root, []byte(policy)); err != nil {
		return errors.New("HLS source cache identity cannot be written")
	}
	return nil
}

// Only an empty root or a previously validated legacy master can be bound.
func hlsSourceBindable(root *os.Root, directory, source, policy string) bool {
	entries, err := fs.ReadDir(root.FS(), ".")
	return err == nil && (len(entries) == 0 || MasterFresh(filepath.Join(directory, "index.m3u8"), source, policy))
}

func hlsSourceFresh(playlist, source, policy string) bool {
	directory := filepath.Dir(playlist)
	if QualityDirectory(filepath.Base(directory)) {
		directory = filepath.Dir(directory)
	}
	root, err := openHLSSourceRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	binding, err := readHLSSource(root)
	if errors.Is(err, os.ErrNotExist) {
		return Fresh(playlist, source)
	}
	if err != nil || policy != "" && string(binding) != policy {
		return false
	}
	expected, valid := hlsPolicySourceVersion(string(binding))
	current, err := hlsSourceVersion(source)
	return valid && err == nil && current == expected
}

func hlsPolicySourceVersion(policy string) (string, bool) {
	if len(policy) > hlsSourcePolicyLimit || strings.ContainsAny(policy, "\r\n") {
		return "", false
	}
	parts := strings.Split(policy, ":")
	if len(parts) < 5 {
		return "", false
	}
	fields := parts[len(parts)-4:]
	if fields[3] != "hls=16" && fields[3] != "hls=15" && fields[3] != "hls=14" && fields[3] != "hls=13" && fields[3] != "hls=7" {
		return "", false
	}
	version, valid := canonicalHLSSourceVersion(fields[0], fields[1])
	if !valid {
		return "", false
	}
	recipe, err := ParseHLSRecipe(fields[2], HLSRecipePolicy{MaxBitrate: math.MaxInt64, OffsetStepMilliseconds: 1})
	if err != nil || recipe.Token() != fields[2] {
		return "", false
	}
	return version, true
}

func canonicalHLSSourceVersion(sizeText, modifiedText string) (string, bool) {
	size, sizeErr := strconv.ParseInt(sizeText, 10, 64)
	modified, timeErr := strconv.ParseInt(modifiedText, 10, 64)
	if sizeErr != nil || timeErr != nil || size <= 0 || strconv.FormatInt(size, 10) != sizeText || strconv.FormatInt(modified, 10) != modifiedText {
		return "", false
	}
	return sizeText + ":" + modifiedText, true
}

func hlsSourceVersion(source string) (string, error) {
	info, err := os.Stat(source) //nolint:gosec // Source comes from the scanned library.
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return "", errors.New("HLS source is not a regular media file")
	}
	return strconv.FormatInt(info.Size(), 10) + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10), nil
}

func openHLSSourceRoot(directory string) (*os.Root, error) {
	parent, err := os.OpenRoot(filepath.Dir(directory)) //nolint:gosec // Directory is an installation cache path with a validated key.
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return parent.OpenRoot(filepath.Base(directory))
}

func readHLSSource(root *os.Root) ([]byte, error) {
	linked, err := root.Lstat(".source")
	if err != nil {
		return nil, err
	}
	if !validHLSSourceFile(linked) {
		return nil, errors.New("HLS source cache identity is invalid")
	}
	file, err := root.Open(".source")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(linked, opened) || !validHLSSourceFile(opened) {
		return nil, errors.New("HLS source cache identity is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, hlsSourcePolicyLimit+1))
	if err != nil || len(data) > hlsSourcePolicyLimit {
		return nil, errors.New("HLS source cache identity is invalid")
	}
	return data, nil
}

func validHLSSourceFile(info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= hlsSourcePolicyLimit
}

func writeHLSSource(root *os.Root, data []byte) error {
	// Exclusive creation and root-relative rename keep every write inside the cache.
	file, err := root.OpenFile(".source.pending", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		_ = root.Remove(".source.pending")
	}()
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return root.Rename(".source.pending", ".source")
}

// ValidateHLSSource distinguishes a changed snapshot from invalid or missing input.
func ValidateHLSSource(source, policy string) error {
	expected, valid := hlsPolicySourceVersion(policy)
	if !valid {
		return errors.New("HLS source policy is invalid")
	}
	current, err := hlsSourceVersion(source)
	if err != nil {
		return err
	}
	if current != expected {
		return ErrHLSSourceChanged
	}
	return nil
}

// StartHLSWorker keeps cancellation and worker completion under one owner.
// Callers defer the returned stop function before publishing or returning output.
func StartHLSWorker(parent context.Context, encode func(context.Context) error) (<-chan error, func()) {
	ctx, cancel := context.WithCancel(parent)
	results := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		results <- encode(ctx)
	}()
	return results, func() {
		cancel()
		<-done
	}
}
