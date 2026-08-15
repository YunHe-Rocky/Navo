param(
    [Parameter(Mandatory)]
    [string]$Path
)

$ErrorActionPreference = "Stop"
$Path = [IO.Path]::GetFullPath($Path)
if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw "File does not exist: $Path"
}

if (-not ("NavoRestartManager" -as [type])) {
    Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Text;

public static class NavoRestartManager {
    [StructLayout(LayoutKind.Sequential)]
    public struct RM_UNIQUE_PROCESS {
        public int ProcessId;
        public System.Runtime.InteropServices.ComTypes.FILETIME ProcessStartTime;
    }

    public enum RM_APP_TYPE {
        Unknown = 0, MainWindow = 1, OtherWindow = 2, Service = 3,
        Explorer = 4, Console = 5, Critical = 1000
    }

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct RM_PROCESS_INFO {
        public RM_UNIQUE_PROCESS Process;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 256)] public string AppName;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 64)] public string ServiceShortName;
        public RM_APP_TYPE ApplicationType;
        public uint ApplicationStatus;
        public uint TerminalSessionId;
        [MarshalAs(UnmanagedType.Bool)] public bool Restartable;
    }

    [DllImport("rstrtmgr.dll", CharSet = CharSet.Unicode)]
    private static extern int RmStartSession(out uint handle, int flags, StringBuilder key);
    [DllImport("rstrtmgr.dll", CharSet = CharSet.Unicode)]
    private static extern int RmRegisterResources(uint handle, uint fileCount, string[] files,
        uint appCount, RM_UNIQUE_PROCESS[] apps, uint serviceCount, string[] services);
    [DllImport("rstrtmgr.dll")]
    private static extern int RmGetList(uint handle, out uint needed, ref uint count,
        [In, Out] RM_PROCESS_INFO[] processes, ref uint rebootReasons);
    [DllImport("rstrtmgr.dll")]
    private static extern int RmEndSession(uint handle);

    public static RM_PROCESS_INFO[] Inspect(string path) {
        uint handle;
        var key = new StringBuilder(33);
        int result = RmStartSession(out handle, 0, key);
        if (result != 0) throw new InvalidOperationException("RmStartSession: " + result);
        try {
            result = RmRegisterResources(handle, 1, new[] { path }, 0, null, 0, null);
            if (result != 0) throw new InvalidOperationException("RmRegisterResources: " + result);
            uint needed = 0, count = 0, reasons = 0;
            result = RmGetList(handle, out needed, ref count, null, ref reasons);
            if (result == 0) return new RM_PROCESS_INFO[0];
            if (result != 234) throw new InvalidOperationException("RmGetList(size): " + result);
            var processes = new RM_PROCESS_INFO[needed];
            count = needed;
            result = RmGetList(handle, out needed, ref count, processes, ref reasons);
            if (result != 0) throw new InvalidOperationException("RmGetList(data): " + result);
            if (count == processes.Length) return processes;
            Array.Resize(ref processes, (int)count);
            return processes;
        } finally {
            RmEndSession(handle);
        }
    }
}
"@
}

[NavoRestartManager]::Inspect($Path) | ForEach-Object {
    $Process = Get-Process -Id $_.Process.ProcessId -ErrorAction SilentlyContinue
    [pscustomobject]@{
        Id = $_.Process.ProcessId
        AppName = $_.AppName
        ApplicationType = [string]$_.ApplicationType
        Restartable = $_.Restartable
        ProcessName = if ($null -ne $Process) { $Process.ProcessName } else { "" }
        StartTime = if ($null -ne $Process) { $Process.StartTime } else { $null }
    }
}
