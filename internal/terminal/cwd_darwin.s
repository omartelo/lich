// Trampoline for the libSystem proc_pidinfo binding in cwd_darwin.go,
// following x/sys/unix's zsyscall_darwin_*.s pattern. Identical on amd64 and
// arm64 — a bare jump to the dynamically imported symbol.

#include "textflag.h"

TEXT libc_proc_pidinfo_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_proc_pidinfo(SB)
GLOBL	·libc_proc_pidinfo_trampoline_addr(SB), RODATA, $8
DATA	·libc_proc_pidinfo_trampoline_addr(SB)/8, $libc_proc_pidinfo_trampoline<>(SB)
