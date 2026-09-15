//go:build linux && (amd64 || arm64)

package publicgateway

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The listener is opened before this filter. All threads may subsequently open
// only Unix sockets. Blocking io_uring and cross-process FD/memory access closes
// alternative paths around socket filtering. The container supplies filesystem,
// PID, privilege and resource isolation; this filter alone is not a sandbox.
func restrictNetwork() error {
	arch := uint32(unix.AUDIT_ARCH_X86_64)
	if runtime.GOARCH == "arm64" {
		arch = unix.AUDIT_ARCH_AARCH64
	}
	filter := networkFilter(arch)
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return errors.New("public gateway privilege restriction failed")
	}
	result, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER, unix.SECCOMP_FILTER_FLAG_TSYNC, uintptr(unsafe.Pointer(&program)))
	runtime.KeepAlive(filter)
	if errno != 0 || result != 0 {
		return errors.New("public gateway network restriction failed")
	}
	return nil
}

func networkFilter(arch uint32) []unix.SockFilter {
	load := func(offset uint32) unix.SockFilter {
		return unix.SockFilter{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: offset}
	}
	equal := func(value uint32, yes, no uint8) unix.SockFilter {
		return unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: value, Jt: yes, Jf: no}
	}
	deny := unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)}
	allow := unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW}
	filter := []unix.SockFilter{load(4), equal(arch, 1, 0), {Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_KILL_PROCESS}, load(0)}
	// Reject the x32 syscall ABI even though it reports AUDIT_ARCH_X86_64.
	filter = append(filter, unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JGE | unix.BPF_K, K: 0x40000000, Jt: 0, Jf: 1}, deny)
	for _, call := range []uint32{unix.SYS_IO_URING_SETUP, unix.SYS_IO_URING_ENTER, unix.SYS_IO_URING_REGISTER, unix.SYS_PTRACE, unix.SYS_PROCESS_VM_READV, unix.SYS_PROCESS_VM_WRITEV, unix.SYS_PIDFD_GETFD, unix.SYS_BPF} {
		filter = append(filter, equal(call, 0, 1), deny)
	}
	filter = append(filter, equal(unix.SYS_SOCKET, 2, 0), equal(unix.SYS_SOCKETPAIR, 1, 0), allow, load(16), equal(unix.AF_UNIX, 1, 0), deny, allow)
	return filter
}
