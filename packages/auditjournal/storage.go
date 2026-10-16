package auditjournal

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

func appendAuditEvent(path string, event Event) error {
	line, _ := json.Marshal(event)
	return appendAuditLine(path, line)
}

func appendAuditLine(path string, line []byte) error {
	if len(line) > maxAuditEventBytes {
		return errors.New("activity event is invalid")
	}
	line = append(line, '\n')
	file, linked, exists, err := openAuditJournal(path)
	if err != nil {
		return err
	}
	opened, err := file.Stat()
	if !appendableJournal(linked, opened, exists, err, int64(len(line))) {
		return rejectAuditJournal(path, file, exists)
	}
	return appendJournalLine(path, file, line, exists, file.Write)
}

func appendableJournal(linked, opened os.FileInfo, existed bool, statErr error, lineLength int64) bool {
	if statErr != nil {
		return false
	}
	if !validOpenedJournal(linked, opened, existed) {
		return false
	}
	return opened.Size()+lineLength <= maxAuditJournalBytes
}

func rejectAuditJournal(path string, file *os.File, existed bool) error {
	_ = file.Close()
	if !existed {
		_ = os.Remove(path)
	}
	return errors.New("activity journal file is invalid")
}

func openAuditJournal(path string) (*os.File, os.FileInfo, bool, error) {
	linked, err := os.Lstat(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, false, err
	}
	flags := os.O_APPEND | os.O_WRONLY
	if !exists {
		flags |= os.O_CREATE | os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o600)
	return file, linked, exists, err
}

func appendJournalLine(path string, file *os.File, line []byte, existed bool, write func([]byte) (int, error)) error {
	return appendJournalLineWithClose(path, file, line, existed, write, file.Close)
}

func appendJournalLineWithClose(path string, file *os.File, line []byte, existed bool, write func([]byte) (int, error), closeFile func() error) error {
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	written, writeErr := write(line)
	if writeErr == nil && written != len(line) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if writeErr != nil {
		rollbackErr := errors.Join(file.Truncate(info.Size()), file.Sync())
		closeErr := closeFile()
		if !existed {
			rollbackErr = errors.Join(rollbackErr, os.Remove(path))
		}
		return errors.Join(writeErr, rollbackErr, closeErr)
	}
	if closeErr := closeFile(); closeErr != nil {
		var rollbackErr error
		if existed {
			rollbackErr = os.Truncate(path, info.Size())
		} else {
			rollbackErr = os.Remove(path)
		}
		return errors.Join(closeErr, rollbackErr)
	}
	return nil
}

func validOpenedJournal(linked, opened os.FileInfo, existed bool) bool {
	if !opened.Mode().IsRegular() {
		return false
	}
	if !securePermissions(opened) {
		return false
	}
	if !existed {
		return true
	}
	if !linked.Mode().IsRegular() {
		return false
	}
	if !securePermissions(linked) {
		return false
	}
	return os.SameFile(linked, opened)
}

func (journal *Journal) enqueueNotification(event Event) {
	event = cloneEvent(event)
	select {
	case journal.notifications <- event:
		return
	default:
	}
	select {
	case <-journal.notifications:
	default:
	}
	journal.notificationDrops.Add(1)
	select {
	case journal.notifications <- event:
	default:
		journal.notificationDrops.Add(1)
	}
}

func (journal *Journal) deliverNotifications(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case event := <-journal.notifications:
			journal.notify(cloneEvent(event))
		}
	}
}
