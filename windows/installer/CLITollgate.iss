; Inno Setup 6 script for CLI Tollgate (per-user install, no admin rights).
; Build: iscc /DAppVersion=0.1.0 /DAppBuild=42 /DNumericVersion=0.1.0.42 /DSourceDir=..\..\build\windows /DOutputDir=..\..\dist windows\installer\CLITollgate.iss
;
; Silent update (used by the app): Setup.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP- [/RELAUNCH=1]

#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
#ifndef AppBuild
  #define AppBuild "0"
#endif
#ifndef NumericVersion
  #define NumericVersion "0.0.0.0"
#endif
#ifndef SourceDir
  #define SourceDir "..\..\build\windows"
#endif
#ifndef OutputDir
  #define OutputDir "..\..\dist"
#endif

#define AppName "CLI Tollgate"
#define AppExe "CLI Tollgate.exe"
#define AppMutex "CLITollgate.Running"

[Setup]
AppId={{6B0E3C1A-7D2F-4C55-9A8E-2F1D7C4B9E10}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher={#AppName}
AppPublisherURL=https://github.com/dylancetin/cli-tollgate
AppSupportURL=https://github.com/dylancetin/cli-tollgate/issues
AppUpdatesURL=https://github.com/dylancetin/cli-tollgate/releases
VersionInfoVersion={#NumericVersion}
PrivilegesRequired=lowest
DefaultDirName={autopf}\{#AppName}
DisableDirPage=yes
DisableProgramGroupPage=yes
DisableReadyPage=yes
DefaultGroupName={#AppName}
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
OutputDir={#OutputDir}
OutputBaseFilename=CLI-Tollgate-Setup-{#AppVersion}
SetupIconFile=..\winres\app.ico
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName={#AppName}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
CloseApplications=no
ShowLanguageDialog=no
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "turkish"; MessagesFile: "compiler:Languages\Turkish.isl"

; Setup follows the Windows display language without asking.
[CustomMessages]
english.AutostartTask=Start {#AppName} when I sign in to Windows
turkish.AutostartTask=Windows'ta oturum açtığımda {#AppName} uygulamasını başlat
english.OptionsGroup=Options:
turkish.OptionsGroup=Seçenekler:
english.AppRunning={#AppName} is running. Setup will close it and continue.
turkish.AppRunning={#AppName} çalışıyor. Kurulum onu kapatıp devam edecek.

[Tasks]
Name: "autostart"; Description: "{cm:AutostartTask}"; GroupDescription: "{cm:OptionsGroup}"

[Files]
Source: "{#SourceDir}\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\tollgate-engine.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\engine-commit.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\CLIProxyAPI-LICENSE.txt"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExe}"

[Registry]
; Same value the app writes from Settings → Launch at sign-in.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{#AppName}"; ValueData: """{app}\{#AppExe}"" --background"; Tasks: autostart; Check: not WizardSilent
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: none; ValueName: "{#AppName}"; Flags: uninsdeletevalue dontcreatekey

[Run]
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent
; Silent updates started by the app ask to be relaunched.
Filename: "{app}\{#AppExe}"; Flags: nowait; Check: ShouldRelaunch

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/F /T /IM ""{#AppExe}"""; Flags: runhidden; RunOnceId: "StopApp"

; User data in %LOCALAPPDATA%\CLI Tollgate (config, sign-ins, logs) is kept on uninstall.

[Code]
function ShouldRelaunch(): Boolean;
begin
  Result := WizardSilent and (ExpandConstant('{param:relaunch|0}') = '1');
end;

// The app starts this installer and then quits; wait for it (and its engine,
// which dies with it) before replacing files. Close it if it is still running.
function InitializeSetup(): Boolean;
var
  I, Code: Integer;
begin
  Result := True;
  I := 0;
  while CheckForMutexes('{#AppMutex}') and (I < 40) do
  begin
    Sleep(250);
    I := I + 1;
  end;
  if CheckForMutexes('{#AppMutex}') then
  begin
    if not WizardSilent then
      if MsgBox(CustomMessage('AppRunning'), mbConfirmation, MB_OKCANCEL) <> IDOK then
      begin
        Result := False;
        Exit;
      end;
    Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /T /IM "{#AppExe}"', '', SW_HIDE, ewWaitUntilTerminated, Code);
    Sleep(500);
  end;
end;
