; lich Windows installer — the Windows counterpart of build/linux/nfpm:
; per-user install (no UAC prompt), Start Menu entry, "Installed apps"
; registration with a working uninstaller. Built by `task package:windows`
; (iscc, Inno Setup 6).

#define AppName "lich"
#define AppExe "lich.exe"
; The version follows the git tag like every package (the Taskfile computes
; and exports it); a local run without one gets a visibly fake version.
#define AppVersion GetEnv("VERSION")
#if AppVersion == ""
  #define AppVersion "0.0.0-dev"
#endif

[Setup]
; Fixed AppId keeps upgrades in place: same id = same install dir and a
; single "Installed apps" entry, updated instead of duplicated.
AppId={{6B3E1DD5-3D8D-4AD8-8476-D8EA84D4CEBE}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=omartelo
AppPublisherURL=https://github.com/omartelo/lich
AppSupportURL=https://github.com/omartelo/lich/issues
; Per-user: lands in %LocalAppData%\Programs\lich, no admin rights involved.
PrivilegesRequired=lowest
DefaultDirName={autopf}\{#AppName}
DisableProgramGroupPage=yes
DisableDirPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\..\bin
OutputBaseFilename=lich-setup
SetupIconFile=lich.ico
UninstallDisplayIcon={app}\{#AppExe}
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
CloseApplicationsFilter=*
RestartApplications=no

[Files]
Source: "..\..\bin\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion
; The window (lich's own Chromium), where lich.exe looks for it: shell\ beside
; the binary (internal/chromium/shell.go). Assembled by `task build:shell`.
Source: "..\..\bin\shell\*"; DestDir: "{app}\shell"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
; The AppUserModelID is the one lich-shell.exe claims for its process
; (shell/src/main.rs): the taskbar then groups the running window under this
; shortcut and draws its icon, pinned or not.
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExe}"; AppUserModelID: "omartelo.lich"

[Run]
Filename: "{app}\{#AppExe}"; Description: "Launch {#AppName}"; Flags: nowait postinstall skipifsilent
; Explicit update mode inherits lich's pinned port and restart marker. Ordinary
; silent installs still do not launch an application on the user's desktop.
Filename: "{app}\{#AppExe}"; Flags: nowait runascurrentuser; Check: IsUpdate

[Code]
const
  SynchronizeAccess = $00100000;
  WaitObject0 = 0;
  ShutdownTimeout = 60000;

function OpenProcess(Access: LongWord; Inherit: Boolean; PID: LongWord): THandle;
  external 'OpenProcess@kernel32.dll stdcall';
function WaitForSingleObject(Handle: THandle; Milliseconds: LongWord): LongWord;
  external 'WaitForSingleObject@kernel32.dll stdcall';
function CloseHandle(Handle: THandle): Boolean;
  external 'CloseHandle@kernel32.dll stdcall';

function IsUpdate: Boolean;
begin
  Result := ExpandConstant('{param:UPDATEPID|0}') <> '0';
end;

function InitializeSetup: Boolean;
var
  Process: THandle;
  PID: Integer;
begin
  Result := True;
  if not IsUpdate then Exit;
  PID := StrToIntDef(ExpandConstant('{param:UPDATEPID|0}'), 0);
  if PID <= 0 then begin
    Result := False;
    Exit;
  end;
  { lich closes its window and flushes its database before releasing its exe.
    Restart Manager then handles any remaining Chromium file holders. }
  Process := OpenProcess(SynchronizeAccess, False, PID);
  if Process <> 0 then begin
    Result := WaitForSingleObject(Process, ShutdownTimeout) = WaitObject0;
    CloseHandle(Process);
  end else
    Result := DLLGetLastError = 87; { ERROR_INVALID_PARAMETER: already exited }
  if Result then
    Log('lich update: outgoing process exited before file replacement')
  else
    MsgBox('lich did not exit. Close lich and run the update again.', mbError, MB_OK);
end;
