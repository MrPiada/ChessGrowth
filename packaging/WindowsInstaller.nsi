Unicode True
Name "ChessGrowth"
Icon "${ICON_PATH}"
UninstallIcon "${ICON_PATH}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\ChessGrowth"
RequestExecutionLevel user
ShowInstDetails show
ShowUninstDetails show
Page directory
Page instfiles
UninstPage uninstConfirm
UninstPage instfiles

Section "ChessGrowth"
 SetOutPath "$INSTDIR"
 File /oname=ChessGrowth.exe "${INPUT_EXE}"
 File /oname=LICENSE "${LICENSE_PATH}"
 CreateDirectory "$SMPROGRAMS\ChessGrowth"
 CreateShortCut "$SMPROGRAMS\ChessGrowth\ChessGrowth.lnk" "$INSTDIR\ChessGrowth.exe"
 CreateShortCut "$SMPROGRAMS\ChessGrowth\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
 WriteUninstaller "$INSTDIR\Uninstall.exe"
 WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ChessGrowth" "DisplayName" "ChessGrowth"
 WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ChessGrowth" "UninstallString" '"$INSTDIR\Uninstall.exe"'
 WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ChessGrowth" "NoModify" 1
 WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ChessGrowth" "NoRepair" 1
SectionEnd

Section "Uninstall"
 Delete "$SMPROGRAMS\ChessGrowth\ChessGrowth.lnk"
 Delete "$SMPROGRAMS\ChessGrowth\Uninstall.lnk"
 RMDir "$SMPROGRAMS\ChessGrowth"
 Delete "$INSTDIR\ChessGrowth.exe"
 Delete "$INSTDIR\LICENSE"
 Delete "$INSTDIR\Uninstall.exe"
 RMDir "$INSTDIR"
 DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\ChessGrowth"
SectionEnd
