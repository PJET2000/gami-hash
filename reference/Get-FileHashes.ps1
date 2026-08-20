<#
.SYNOPSIS
  GAMI reference hashing script — one page, for institutions with IT staff.

  Produces the same CSV as the GAMI Hashing Tool:
    relative_path,filename,size_bytes,sha256,mtime_utc
  (UTF-8 with BOM, CRLF, RFC 4180). It can therefore also be used to
  independently cross-check the tool's output on a sample.

.EXAMPLE
  .\Get-FileHashes.ps1 -Root "E:\Bestand" -Output "C:\Temp\pruefsummen.csv"

.NOTES
  - Reads only; writes only the output CSV (refused inside -Root).
  - Requires Windows PowerShell 5.1+ or PowerShell 7+. No admin rights.
  - Row order can differ slightly from the tool's (both are deterministic);
    compare as a set. Unreadable files are reported on stderr and skipped.
#>
param(
  [Parameter(Mandatory)][string]$Root,
  [Parameter(Mandatory)][string]$Output
)
$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path -LiteralPath $Root).Path.TrimEnd('\')
$OutFull = [IO.Path]::GetFullPath($Output)
if ($OutFull.ToLower().StartsWith(($Root + '\').ToLower())) {
  throw "Output file must not be inside the scanned folder."
}

function Csv-Field([string]$s) {
  if ($s -match '[",\r\n]') { '"' + ($s -replace '"', '""') + '"' } else { $s }
}

$writer = New-Object IO.StreamWriter($OutFull, $false, (New-Object Text.UTF8Encoding($true)))
$writer.NewLine = "`r`n"
$writer.WriteLine('relative_path,filename,size_bytes,sha256,mtime_utc')

$files = Get-ChildItem -LiteralPath $Root -Recurse -File -Force |
  Where-Object { -not ($_.Attributes -band [IO.FileAttributes]::ReparsePoint) } |
  Sort-Object -Property @{ Expression = { $_.FullName.Substring($Root.Length + 1) } } -Culture ''

$done = 0; $failed = 0
foreach ($f in $files) {
  $rel = $f.FullName.Substring($Root.Length + 1).Replace('\', '/')
  try {
    $hash = (Get-FileHash -LiteralPath $f.FullName -Algorithm SHA256).Hash.ToLower()
    $mtime = $f.LastWriteTimeUtc.ToString("yyyy-MM-dd'T'HH:mm:ss'Z'")
    $writer.WriteLine(('{0},{1},{2},{3},{4}' -f (Csv-Field $rel), (Csv-Field $f.Name), $f.Length, $hash, $mtime))
    $done++
  } catch {
    [Console]::Error.WriteLine("SKIPPED (unreadable): $rel — $($_.Exception.Message)")
    $failed++
  }
  if (($done % 500) -eq 0) { Write-Progress -Activity 'Hashing' -Status "$done files done" }
}
$writer.Close()
Write-Host "Done: $done files recorded in $OutFull ($failed unreadable, skipped)."
