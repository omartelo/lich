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

[Files]
Source: "..\..\bin\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExe}"

[Run]
Filename: "{app}\{#AppExe}"; Description: "Launch {#AppName}"; Flags: nowait postinstall skipifsilent
