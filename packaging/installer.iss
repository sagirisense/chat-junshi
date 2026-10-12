#define AppVersion "0.1.0"
[Setup]
AppId={{447E6C02-E9B1-4E87-84BA-942BB5399E4E}
AppName=聊天军师
AppVersion={#AppVersion}
AppPublisher=Chat Junshi contributors
AppPublisherURL=https://github.com/sagirisense/chat-junshi
DefaultDirName={localappdata}\Programs\ChatJunshi
DefaultGroupName=聊天军师
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.18362
OutputDir=..\release
OutputBaseFilename=ChatJunshi-Setup-{#AppVersion}-x64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
LicenseFile=..\LICENSE
UninstallDisplayIcon={app}\chat-junshi.exe
CloseApplications=yes
[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; Flags: unchecked
[Files]
Source: "dist\ChatJunshi\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
[Icons]
Name: "{group}\聊天军师"; Filename: "{app}\chat-junshi.exe"
Name: "{group}\卸载聊天军师"; Filename: "{uninstallexe}"
Name: "{autodesktop}\聊天军师"; Filename: "{app}\chat-junshi.exe"; Tasks: desktopicon
[Run]
Filename: "{app}\chat-junshi.exe"; Description: "Launch Chat Junshi"; Flags: nowait postinstall skipifsilent
[Code]
function InitializeSetup(): Boolean;
var RuntimeVersion: String;
begin
 Result := RegQueryStringValue(HKLM, 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', RuntimeVersion)
   or RegQueryStringValue(HKCU, 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', RuntimeVersion);
 if not Result then
 begin
   MsgBox('Microsoft Edge WebView2 Runtime is required. Install it from https://developer.microsoft.com/microsoft-edge/webview2/ then run this installer again.', mbInformation, MB_OK);
 end;
end;
