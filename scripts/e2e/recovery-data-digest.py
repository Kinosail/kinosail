"""Hash stopped fixture data using only the supervisor's inherited root capability."""
import hashlib
import os
import stat
import sys

FLAGS = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
LIMIT = 33554432

def identity(value):
    return value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns, value.st_ctime_ns

def data_digest(root_fd):
    root = os.fstat(root_fd)
    if not stat.S_ISDIR(root.st_mode) or root.st_uid != os.getuid() or stat.S_IMODE(root.st_mode) != 0o700:
        raise ValueError('invalid root')
    digest = hashlib.sha256()
    count = total = 0

    def walk(fd, prefix):
        nonlocal count, total
        before = os.fstat(fd)
        entries = []
        with os.scandir(fd) as directory:
            for entry in directory:
                count += 1
                if count > 256:
                    raise ValueError('entry limit')
                entries.append((entry.name, entry.inode()))
        for name, inode in sorted(entries, key=lambda row: row[0].encode('utf-16-be', 'surrogatepass')):
            # A single directory entry is never an ancestor pathname.
            if name in ('.', '..') or '/' in name or '\0' in name:
                raise ValueError('invalid entry')
            child = os.open(name, FLAGS, dir_fd=fd)
            try:
                opened = os.fstat(child)
                if opened.st_ino != inode or opened.st_dev != before.st_dev or opened.st_uid != root.st_uid:
                    raise ValueError('entry replaced')
                if stat.S_ISDIR(opened.st_mode):
                    walk(child, prefix + name + '/')
                elif stat.S_ISREG(opened.st_mode):
                    if opened.st_nlink != 1:
                        raise ValueError('linked file')
                    total += opened.st_size
                    if opened.st_size < 0 or total > LIMIT:
                        raise ValueError('byte limit')
                    digest.update((prefix + name + '\0').encode('utf-8', 'strict'))
                    remaining = opened.st_size
                    while remaining:
                        if identity(os.fstat(child)) != identity(opened):
                            raise ValueError('file changed')
                        body = os.read(child, min(65536, remaining))
                        if not body:
                            raise ValueError('file shortened')
                        remaining -= len(body)
                        digest.update(body)
                    if identity(os.fstat(child)) != identity(opened):
                        raise ValueError('file changed')
                    digest.update(b'\0')
                else:
                    raise ValueError('invalid entry type')
                current = os.stat(name, dir_fd=fd, follow_symlinks=False)
                if identity(current) != identity(opened):
                    raise ValueError('entry changed')
            finally:
                os.close(child)
        if identity(os.fstat(fd)) != identity(before):
            raise ValueError('directory changed')

    data = os.open('data', FLAGS | os.O_DIRECTORY, dir_fd=root_fd)
    try:
        if os.fstat(data).st_uid != root.st_uid:
            raise ValueError('invalid data owner')
        walk(data, '')
        current = os.stat('data', dir_fd=root_fd, follow_symlinks=False)
        if identity(current) != identity(os.fstat(data)):
            raise ValueError('data changed')
    finally:
        os.close(data)
    return digest.hexdigest()

if __name__ == '__main__':
    try:
        if len(sys.argv) != 1:
            raise ValueError('unexpected arguments')
        result = data_digest(3)
    except Exception:
        sys.exit(1)
    print(result)
