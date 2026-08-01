param(
    [Parameter(Mandatory)]
    [int]$ProcessId,
    [int]$TimeoutSeconds = 15
)

$ErrorActionPreference = "Stop"

if (-not ("NavoTrayExitProbe" -as [type])) {
    Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class NavoTrayExitProbe {
    [StructLayout(LayoutKind.Sequential)]
    public struct Rect {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct Point {
        public int X;
        public int Y;
    }

    [DllImport("user32.dll", CharSet=CharSet.Unicode)]
    public static extern IntPtr FindWindow(string className, string windowTitle);
    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr window, out uint processId);
    [DllImport("user32.dll")]
    public static extern bool PostMessage(IntPtr window, uint message, UIntPtr wordParameter, IntPtr longParameter);
    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr window, out Rect rectangle);
    [DllImport("user32.dll")]
    public static extern bool GetCursorPos(out Point point);
    [DllImport("user32.dll")]
    public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")]
    public static extern void mouse_event(uint flags, uint x, uint y, uint data, UIntPtr extraInfo);
}
"@
}

$process = Get-Process -Id $ProcessId -ErrorAction Stop
$trayWindow = [IntPtr]::Zero
for ($attempt = 0; $attempt -lt 50; $attempt++) {
    $candidate = [NavoTrayExitProbe]::FindWindow("NavoTrayClass", $null)
    if ($candidate -ne [IntPtr]::Zero) {
        $owner = [uint32]0
        [NavoTrayExitProbe]::GetWindowThreadProcessId($candidate, [ref]$owner) | Out-Null
        if ($owner -eq $ProcessId) {
            $trayWindow = $candidate
            break
        }
    }
    Start-Sleep -Milliseconds 100
}
if ($trayWindow -eq [IntPtr]::Zero) {
    throw "Navo tray window for PID $ProcessId was not found"
}

$cursor = New-Object NavoTrayExitProbe+Point
[NavoTrayExitProbe]::GetCursorPos([ref]$cursor) | Out-Null
try {
    if (-not [NavoTrayExitProbe]::PostMessage(
        $trayWindow, 0x8001, [UIntPtr]::Zero, [IntPtr]0x0205
    )) {
        throw "Unable to open Navo tray menu for PID $ProcessId"
    }

    $menuWindow = [IntPtr]::Zero
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        Start-Sleep -Milliseconds 100
        $candidate = [NavoTrayExitProbe]::FindWindow("#32768", $null)
        if ($candidate -eq [IntPtr]::Zero) {
            continue
        }
        $owner = [uint32]0
        [NavoTrayExitProbe]::GetWindowThreadProcessId($candidate, [ref]$owner) | Out-Null
        if ($owner -eq $ProcessId) {
            $menuWindow = $candidate
            break
        }
    }
    if ($menuWindow -eq [IntPtr]::Zero) {
        throw "Navo tray menu for PID $ProcessId was not found"
    }

    $menuRect = New-Object NavoTrayExitProbe+Rect
    if (-not [NavoTrayExitProbe]::GetWindowRect($menuWindow, [ref]$menuRect)) {
        throw "Unable to inspect Navo tray menu for PID $ProcessId"
    }
    [NavoTrayExitProbe]::SetCursorPos(
        [int](($menuRect.Left + $menuRect.Right) / 2),
        $menuRect.Bottom - 16
    ) | Out-Null
    [NavoTrayExitProbe]::mouse_event(0x0002, 0, 0, 0, [UIntPtr]::Zero)
    [NavoTrayExitProbe]::mouse_event(0x0004, 0, 0, 0, [UIntPtr]::Zero)
} finally {
    [NavoTrayExitProbe]::SetCursorPos($cursor.X, $cursor.Y) | Out-Null
}

if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
    throw "Navo PID $ProcessId did not exit through the tray within $TimeoutSeconds seconds"
}
if ($process.ExitCode -ne 0) {
    throw "Navo PID $ProcessId exited with code $($process.ExitCode)"
}
