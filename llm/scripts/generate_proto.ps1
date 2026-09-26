$ErrorActionPreference = "Stop"

$serviceRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$projectRoot = Resolve-Path (Join-Path $serviceRoot "..")
$protoRoot = Join-Path $projectRoot "proto"
$outputRoot = Join-Path $serviceRoot "src"
$pythonExecutable = Join-Path $serviceRoot ".venv\Scripts\python.exe"

if (-not (Test-Path -LiteralPath $pythonExecutable)) {
    throw "LLM virtual environment was not found: $pythonExecutable"
}

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null

& $pythonExecutable -m grpc_tools.protoc `
    --proto_path=$protoRoot `
    --python_out=$outputRoot `
    --pyi_out=$outputRoot `
    --grpc_python_out=$outputRoot `
    (Join-Path $protoRoot "llm\v1\llm.proto")

if ($LASTEXITCODE -ne 0) {
    throw "failed to generate Python gRPC code"
}

Write-Host "Python gRPC code generated in $(Join-Path $outputRoot 'llm\v1')"
