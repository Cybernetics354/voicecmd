package audio

// #cgo LDFLAGS: -lasound
// #include <alsa/asoundlib.h>
// static void null_alsa_handler(const char *file, int line, const char *function, int err, const char *fmt, ...) {}
// static void suppress_alsa_logging(void) {
//     snd_lib_error_set_handler(null_alsa_handler);
// }
import "C"

func init() {
	C.suppress_alsa_logging()
}
