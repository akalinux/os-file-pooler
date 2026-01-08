// Get Shell Exit code
package sec

/*
#include <signal.h>
int process_exit_info(siginfo_t *info) {

    switch (info->si_code) {
        case CLD_EXITED:
            // si_status is the actual exit(n) value
            return  info->si_status;

        case CLD_KILLED:
        case CLD_DUMPED:
            // si_status is the signal number (e.g., 9)
            // We add 128 to match shell convention (e.g., 137)
            return 128 + info->si_status;
        default:
			return -1;
    }
}
*/
import "C"
import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func GetExitCodeFromSigInfo(src *unix.Siginfo) int {
	info := (*C.siginfo_t)(unsafe.Pointer(src))
	code := C.process_exit_info(info)
	return int(code)
}
