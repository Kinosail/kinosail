package main

import (
 "errors"
 "io"
 "os"
)

const fixtureTarget = "media/R06 Fictional Save.en.srt"

func prepareFixtureFiles(directory string) (*os.Root, error) {
 root, err := os.OpenRoot(directory)
 if err != nil { return nil, errors.New("fixture directory unavailable") }
 if err = initializeFixtureFiles(root); err != nil {
  return nil, errors.Join(err, root.Close())
 }
 return root, nil
}

func initializeFixtureFiles(root *os.Root) error {
 directory, err := root.Open(".")
 if err != nil { return errors.New("fixture directory unavailable") }
 entries, readErr := directory.ReadDir(-1)
 if err = errors.Join(readErr, directory.Close()); err != nil || len(entries) != 0 {
  return errors.New("fixture directory must be empty")
 }
 for _, name := range []string{"media", "data", "cache"} {
  if err = root.Mkdir(name, 0700); err != nil { return errors.New("fixture directory unavailable") }
 }
 if err = root.WriteFile("media/R06 Fictional Save.mp4", []byte("R06 Save-only indexed fixture; no decoded media"), 0600); err != nil {
  return errors.New("fictional fixture unavailable")
 }
 if err = root.WriteFile(fixtureTarget, []byte(initialSRT), 0600); err != nil {
  return errors.New("fictional fixture unavailable")
 }
 return nil
}

func (f *fixture) readFixtureSubtitle(backup bool) ([]byte, error) {
 name := fixtureTarget
 if backup { name += ".kinosail.bak" }
 file, err := f.files.Open(name)
 if err != nil { return nil, err }
 data, readErr := io.ReadAll(io.LimitReader(file, privateResponseLimit+1))
 err = errors.Join(readErr, file.Close())
 if len(data) > privateResponseLimit { err = errors.Join(err, errors.New("fixture subtitle bound exceeded")) }
 return data, err
}
