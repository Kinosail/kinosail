package server

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type copiedHLSWindowsProbe struct {
	job     windows.Handle
	process windows.ProcessInformation
}

// WinSDK WinBase.h: ProcThreadAttributeJobList (13) | INPUT (0x20000).
// https://github.com/microsoft/win32metadata/blob/main/generation/WinSDK/RecompiledIdlHeaders/um/WinBase.h
const copiedHLSJobListAttribute = 0x2000d

func startCopiedHLSProbe(ctx context.Context, executable string, arguments []string) (copiedHLSProbe, io.ReadCloser, error) {
	if ctx.Err() != nil {
		return nil, nil, errCopiedHLSIndex
	}
	//nolint:gosec // Executable is installation config; media comes from a revalidated scanned item.
	command := exec.Command(executable, arguments...)
	if command.Err != nil {
		return nil, nil, errCopiedHLSIndex
	}
	path, err := exec.LookPath(command.Path)
	if err != nil {
		return nil, nil, errCopiedHLSIndex
	}
	job, err := copiedHLSWindowsJob()
	if err != nil {
		return nil, nil, errCopiedHLSIndex
	}
	probe, output, err := copiedHLSWindowsLaunch(ctx, job, path, command.Args)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, nil, errCopiedHLSIndex
	}
	return probe, output, nil
}

func copiedHLSWindowsJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func copiedHLSWindowsLaunch(ctx context.Context, job windows.Handle, path string, arguments []string) (copiedHLSProbe, io.ReadCloser, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	defer writer.Close()
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		_ = reader.Close()
		return nil, nil, err
	}
	defer null.Close()
	handles, err := copiedHLSWindowsHandles(writer, null)
	if err != nil {
		_ = reader.Close()
		return nil, nil, err
	}
	defer func() {
		for _, handle := range handles {
			_ = windows.CloseHandle(handle)
		}
	}()
	process, err := copiedHLSWindowsCreate(ctx, job, path, arguments, handles)
	if err != nil {
		_ = reader.Close()
		return nil, nil, err
	}
	return &copiedHLSWindowsProbe{job: job, process: process}, reader, nil
}

func copiedHLSWindowsHandles(output, null *os.File) ([]windows.Handle, error) {
	current := windows.CurrentProcess()
	handles := make([]windows.Handle, 0, 2)
	for _, file := range []*os.File{output, null} {
		var handle windows.Handle
		if err := windows.DuplicateHandle(current, windows.Handle(file.Fd()), current, &handle, 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
			for _, previous := range handles {
				_ = windows.CloseHandle(previous)
			}
			return nil, err
		}
		handles = append(handles, handle)
	}
	return handles, nil
}

func copiedHLSWindowsCreate(ctx context.Context, job windows.Handle, path string, arguments []string, handles []windows.Handle) (windows.ProcessInformation, error) {
	var process windows.ProcessInformation
	application, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return process, err
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(arguments))
	if err != nil {
		return process, err
	}
	attributes, err := windows.NewProcThreadAttributeList(2)
	if err != nil {
		return process, err
	}
	defer attributes.Delete()
	jobs := []windows.Handle{job}
	if err = attributes.Update(copiedHLSJobListAttribute, unsafe.Pointer(&jobs[0]), unsafe.Sizeof(job)); err != nil {
		return process, err
	}
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return process, err
	}
	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdInput, startup.StdOutput, startup.StdErr = handles[1], handles[0], handles[1]
	startup.ProcThreadAttributeList = attributes.List()
	if ctx.Err() != nil {
		return process, errCopiedHLSIndex
	}
	// Membership is atomic with creation; no unmanaged child or breakaway fallback.
	err = windows.CreateProcess(application, line, nil, nil, true, windows.EXTENDED_STARTUPINFO_PRESENT, nil, nil, &startup.StartupInfo, &process)
	runtime.KeepAlive(jobs)
	runtime.KeepAlive(handles)
	return process, err
}

func (probe *copiedHLSWindowsProbe) exited() (bool, error) {
	result, err := windows.WaitForSingleObject(probe.process.Process, 0)
	return result == windows.WAIT_OBJECT_0, err
}

func (probe *copiedHLSWindowsProbe) terminate() error {
	return windows.TerminateJobObject(probe.job, 1)
}

func (probe *copiedHLSWindowsProbe) wait() error {
	if _, err := windows.WaitForSingleObject(probe.process.Process, windows.INFINITE); err != nil {
		return err
	}
	var code uint32
	if windows.GetExitCodeProcess(probe.process.Process, &code) != nil || code != 0 {
		return errCopiedHLSIndex
	}
	return nil
}

// ABI-defined JOBOBJECT_BASIC_ACCOUNTING_INFORMATION, including ActiveProcesses.
type copiedHLSJobAccounting struct {
	totalUserTime       int64
	totalKernelTime     int64
	periodUserTime      int64
	periodKernelTime    int64
	totalPageFaults     uint32
	totalProcesses     uint32
	activeProcesses    uint32
	terminatedProcesses uint32
}

func (probe *copiedHLSWindowsProbe) settled() (bool, error) {
	var accounting copiedHLSJobAccounting
	err := windows.QueryInformationJobObject(probe.job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&accounting)), uint32(unsafe.Sizeof(accounting)), nil)
	return accounting.activeProcesses == 0 && err == nil, err
}

func (probe *copiedHLSWindowsProbe) close() {
	_ = windows.CloseHandle(probe.process.Thread)
	_ = windows.CloseHandle(probe.process.Process)
	_ = windows.CloseHandle(probe.job)
}
