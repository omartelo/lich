package terminal

// windowsShells are the shells a Windows "shell" session is opened in, best
// first: PowerShell 7 when the user installed it, else the 5.1 that has shipped
// with Windows since 7. The first one on $PATH wins (userShell).
//
// cmd.exe is deliberately absent, and COMSPEC — which names it — is deliberately
// not read. COMSPEC is set on every Windows box and always points at cmd.exe, so
// honouring it the way $SHELL is honoured on Unix would mean never opening
// anything else: it is not a shell the user chose, it is the one Windows ships.
// cmd.exe is still what runs a .cmd or .bat provider shim (wrapArgv) — that is
// CreateProcess' limitation, not a shell the user is ever dropped into.
//
// Kept out of the build-tagged file so the order is tested on any OS, the
// pattern wrapArgv and dosPathRunes follow.
var windowsShells = []string{"pwsh.exe", "powershell.exe"}
