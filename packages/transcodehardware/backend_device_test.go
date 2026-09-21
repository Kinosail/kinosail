package transcodehardware

import "testing"

func TestBackendDeviceRequiresMatchingHardwareKind(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ backend, device, want string }{
		{"vaapi", "/dev/dri/renderD128", "/dev/dri/renderD128"},
		{"qsv", "/dev/dri/renderD999999", "/dev/dri/renderD999999"},
		{"rkmpp", "/dev/dri/renderD128", "/dev/dri/renderD128"},
		{"cuda", "/dev/nvidia0", "0"},
		{"cuda", "/dev/nvidia12", "12"},
		{"cuda", "/dev/nvidiactl", ""},
		{"cuda", "/dev/nvidia-uvm", ""},
		{"cuda", "/dev/dri/renderD128", ""},
		{"vaapi", "/dev/nvidia0", ""},
		{"none", "/dev/dri/renderD128", ""},
		{"qsv", "", ""},
	} {
		if got := backendDevice(test.backend, test.device); got != test.want {
			t.Errorf("%s %s = %q, want %q", test.backend, test.device, got, test.want)
		}
	}
}
