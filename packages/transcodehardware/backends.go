package transcodehardware

type hardwareDefinition struct {
	id, name, encoder, hw string
	device, supported     bool
	relevant              bool
	reason, action        string
}

func hardwareDefinitions(goos, goarch string, hasDevice func(...string) bool, platform string) []hardwareDefinition { //nolint:cyclop,gocognit // Platform capability mapping stays explicit and fail-closed.
	return []hardwareDefinition{
		{id: "qsv", name: "Intel Quick Sync", encoder: "h264_qsv", hw: "qsv", device: goos == "windows" || hasDevice("render"), supported: qsvSupported(goos, goarch), reason: "Use an x86-64 Linux or Windows Server.", action: "Make /dev/dri available on the host, then rerun the installer. It adds container access automatically."},
		{id: "cuda", name: "NVIDIA NVENC/NVDEC", encoder: "h264_nvenc", hw: "cuda", device: goos == "windows" || hasDevice("nvidia"), supported: cudaSupported(goos, goarch), reason: "Use a Linux or x86-64 Windows Server.", action: "Install the NVIDIA driver and Container Toolkit, then rerun the installer."},
		{id: "vaapi", name: "AMD/Intel VA-API", encoder: "h264_vaapi", hw: "vaapi", device: hasDevice("render"), supported: vaapiSupported(goos, goarch), reason: "Use an x86-64 or Arm64 Linux or FreeBSD Server.", action: "Make /dev/dri available on the host, then rerun the installer. It adds container access automatically."},
		{id: "rkmpp", name: "Rockchip RKMPP", encoder: "h264_rkmpp", hw: "rkmpp", device: hasDevices(hasDevice, "render", "dma_heap", "mpp", "rga"), supported: goos == "linux" && goarch == "arm64", reason: "Use an Arm64 Linux Server.", action: "Make the Rockchip video devices available on the host, then rerun the installer."},
		{id: "v4l2m2m", name: "Linux V4L2 hardware encoder", encoder: "h264_v4l2m2m", device: hasDevice("video"), supported: linuxSupported(goos, goarch), reason: "Use a Linux Server.", action: "Set KINOSAIL_GPU_BACKEND=device and map the encoder, such as /dev/video11:/dev/video11, then rerun the installer."},
		{id: "videotoolbox", name: "Apple VideoToolbox", encoder: "h264_videotoolbox", hw: "videotoolbox", device: true, supported: goos == "darwin", reason: "Use a native macOS Server. Linux containers cannot access VideoToolbox."},
		{id: "amf", name: "AMD AMF", encoder: "h264_amf", hw: "d3d11va", device: true, supported: goos == "windows" && goarch == "amd64", reason: "Use a native x86-64 Windows Server. Linux AMD GPUs use VA-API."},
		{id: "mf", name: "Windows Media Foundation", encoder: "h264_mf", device: goos == "windows", supported: windowsSupported(goos, goarch), reason: "Use a native Windows Server."},
		{id: "asahi", name: "Apple Silicon on Asahi Linux", relevant: platform == "asahi", reason: "Asahi Linux does not yet provide Apple hardware video encoding."},
	}
}

func hasDevices(hasDevice func(...string) bool, names ...string) bool {
	for _, name := range names {
		if !hasDevice(name) {
			return false
		}
	}
	return true
}

func qsvSupported(goos, goarch string) bool {
	return goarch == "amd64" && (goos == "linux" || goos == "windows")
}

func cudaSupported(goos, goarch string) bool {
	return (goos == "linux" && (goarch == "amd64" || goarch == "arm64")) || (goos == "windows" && goarch == "amd64")
}

func vaapiSupported(goos, goarch string) bool {
	return (goos == "linux" || goos == "freebsd") && (goarch == "amd64" || goarch == "arm64")
}

func linuxSupported(goos, goarch string) bool {
	return goos == "linux" && (goarch == "amd64" || goarch == "arm64")
}

func windowsSupported(goos, goarch string) bool {
	return goos == "windows" && (goarch == "amd64" || goarch == "arm64")
}

func hardwareMessage(definition hardwareDefinition, application string, probed, detected, usable bool) (string, string, string) {
	switch definition.id { //nolint:gocritic // Explicit value dispatch keeps mutation coverage deterministic.
	case "asahi":
		return "Software encoding only", definition.reason, "Keep Automatic selected. Kinosail will use its software encoders."
	}
	switch {
	case !definition.supported:
		return "Different system", definition.reason, "Keep Automatic selected. Kinosail will use a backend for this system."
	case !probed:
		return "Detection off", application + " did not inspect this FFmpeg installation.", "Enable detection or run the local transcoder test."
	case !detected:
		return "FFmpeg update needed", definition.name + " is not included in this FFmpeg build.", "Install an FFmpeg build that includes " + definition.encoder + "."
	case !usable:
		return "Device access needed", "FFmpeg includes this backend, but Kinosail cannot access its hardware device.", definition.action
	default:
		return "Ready to test", "Kinosail found the FFmpeg encoder and its device interface.", "Run the local transcoder test to confirm this device."
	}
}
