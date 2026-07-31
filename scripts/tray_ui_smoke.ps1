param(
    [string]$ScreenshotPath = ""
)

$ErrorActionPreference = "Stop"
$ProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$ReleaseRoot = Join-Path $ProjectRoot "release\Navo"
$Executable = Join-Path $ReleaseRoot "navo.exe"
if ($ScreenshotPath -eq "") {
    $ScreenshotPath = Join-Path $ProjectRoot "release\tray-menu-smoke.png"
}
if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) {
    throw "Packaged Navo executable not found: $Executable"
}

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class NavoTrayProbe {
    [StructLayout(LayoutKind.Sequential)]
    public struct Rect {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }

    [DllImport("user32.dll", CharSet=CharSet.Unicode)]
    public static extern IntPtr FindWindow(string className, string windowTitle);

    [DllImport("user32.dll")]
    public static extern bool PostMessage(
        IntPtr window,
        uint message,
        UIntPtr wordParameter,
        IntPtr longParameter
    );

    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr window, out Rect rectangle);

    [DllImport("user32.dll")]
    public static extern bool SetCursorPos(int x, int y);

    [DllImport("user32.dll")]
    public static extern void mouse_event(
        uint flags,
        uint x,
        uint y,
        uint data,
        UIntPtr extraInfo
    );
}
"@

$process = $null
try {
    $process = Start-Process `
        -FilePath $Executable `
        -WorkingDirectory $ReleaseRoot `
        -WindowStyle Hidden `
        -PassThru

    $trayWindow = [IntPtr]::Zero
    for ($attempt = 0; $attempt -lt 50 -and $trayWindow -eq [IntPtr]::Zero; $attempt++) {
        Start-Sleep -Milliseconds 200
        $trayWindow = [NavoTrayProbe]::FindWindow("NavoTrayClass", $null)
    }
    if ($trayWindow -eq [IntPtr]::Zero) {
        throw "Navo tray window was not created"
    }

    # NOTIFYICON_VERSION_4 sends the mouse event in the low word of lParam.
    $posted = [NavoTrayProbe]::PostMessage(
        $trayWindow,
        0x8001,
        [UIntPtr]::Zero,
        [IntPtr]0x0205
    )
    if (-not $posted) {
        throw "Unable to open the Navo tray menu"
    }
    Start-Sleep -Milliseconds 800

    $bounds = [System.Windows.Forms.SystemInformation]::VirtualScreen
    $bitmap = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
        $graphics.CopyFromScreen(
            $bounds.Left,
            $bounds.Top,
            0,
            0,
            $bitmap.Size
        )
        $bitmap.Save(
            $ScreenshotPath,
            [System.Drawing.Imaging.ImageFormat]::Png
        )
    }
    finally {
        $graphics.Dispose()
        $bitmap.Dispose()
    }

    # Exit is the final root-menu command. This validates the real callback and
    # launcher shutdown path instead of forcibly terminating the test process.
    $menuWindow = [NavoTrayProbe]::FindWindow("#32768", $null)
    if ($menuWindow -eq [IntPtr]::Zero) {
        throw "Native tray popup menu window was not found"
    }
    $menuRect = New-Object NavoTrayProbe+Rect
    if (-not [NavoTrayProbe]::GetWindowRect($menuWindow, [ref]$menuRect)) {
        throw "Unable to inspect the native tray popup menu"
    }
    $exitX = [int](($menuRect.Left + $menuRect.Right) / 2)
    $exitY = $menuRect.Bottom - 16
    [NavoTrayProbe]::SetCursorPos($exitX, $exitY) | Out-Null
    [NavoTrayProbe]::mouse_event(0x0002, 0, 0, 0, [UIntPtr]::Zero)
    [NavoTrayProbe]::mouse_event(0x0004, 0, 0, 0, [UIntPtr]::Zero)
    if (-not $process.WaitForExit(15000)) {
        throw "Tray Exit did not stop Navo within 15 seconds"
    }
    if ($process.ExitCode -ne 0) {
        throw "Navo exited with code $($process.ExitCode)"
    }

    [ordered]@{
        tray_window = $trayWindow.ToInt64()
        exit_code = $process.ExitCode
        screenshot = $ScreenshotPath
    } | ConvertTo-Json
}
finally {
    if ($null -ne $process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force
    }
}
