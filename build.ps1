param(
    [string]$GoImage = "golang:1.23",
    [string]$Output = "wslc-compose.exe"
)

$ErrorActionPreference = "Stop"
$src = (Get-Location).Path

wslc run --rm -v "${src}:/src" -w /src $GoImage sh -c "go mod tidy && GOOS=windows GOARCH=amd64 go build -o $Output ."
if ($LASTEXITCODE -ne 0) {
    throw "build failed"
}
Write-Host "Built $Output"
