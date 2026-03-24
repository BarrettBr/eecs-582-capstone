# Windows Startup Guide

This guide is the recommended way to run the project locally on Windows.

Use this guide if:

- you are developing on Windows 10 or 11
- you want to run the frontend, ingest service, and ML service together
- `make dev` or Git Bash / `MINGW64` gave you path or Python issues

## Recommended Shell

Use `PowerShell` for the local Windows workflow.

Why:

- the repo's `Makefile` assumes Unix-style paths such as `.venv/bin/python`
- native Windows virtual environments use `.venv\Scripts\python.exe`
- Python package installs are more reliable from PowerShell on Windows

## Prerequisites

Install these before starting:

- `Node.js`
  - see `frontend/package.json` for the supported version range
- `Go`
- `Python 3.12` or `Python 3.13`
  - `Python 3.14` may fail for this project because some ML dependencies may not have stable Windows wheels yet

To check available Python installs:

```powershell
py -0
```

## One-Time Setup

Open `PowerShell` in the repository root and run:

```powershell
Remove-Item -Recurse -Force .venv
py -3.12 -m venv .venv
.\.venv\Scripts\python.exe -m pip install --upgrade pip
.\.venv\Scripts\python.exe -m pip install -r ml\app\requirements.txt
cd .\frontend
npm install
cd ..
```

If you prefer Python `3.13`, replace `py -3.12` with `py -3.13`.

## Start The Full Local Stack

From the repository root:

```powershell
powershell -ExecutionPolicy Bypass -File .\dev-stack.ps1
```

That script opens three PowerShell windows for:

- the ML API on `http://127.0.0.1:8000`
- the ingest API and websocket server on `http://127.0.0.1:8080`
- the Vite frontend on `http://localhost:5173`

Then open:

- Frontend: `http://localhost:5173`
- Ingest ping: `http://127.0.0.1:8080/api/v1/ping`
- ML health: `http://127.0.0.1:8000/health`

## Start Modbus Mode

To run with the checked-in `modbus` profile instead of simulator mode:

```powershell
powershell -ExecutionPolicy Bypass -File .\dev-stack.ps1 -Modbus
```

The default startup without `-Modbus` is simulator mode.

## Optional PowerShell Setting

If PowerShell blocks local scripts with an execution policy error, you can either keep using the temporary bypass command above or set a friendlier user-level policy once:

```powershell
Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
```

After that, you can usually start the stack with:

```powershell
.\dev-stack.ps1
```

## Manual Startup

If you want to run each service yourself, use three PowerShell windows.

ML service:

```powershell
cd "C:\Users\Minh Vu\Desktop\Spring 2026\eecs582\eecs-582-capstone"
.\.venv\Scripts\python.exe ml\app\main.py --serve --host 127.0.0.1 --port 8000
```

Ingest service:

```powershell
cd "C:\Users\Minh Vu\Desktop\Spring 2026\eecs582\eecs-582-capstone\ingest"
$env:SOURCE_CONFIG_PATH="config/sources.json"
$env:ML_API_URL="http://127.0.0.1:8000"
$env:ML_HTTP_TIMEOUT="5s"
go run .
```

Frontend:

```powershell
cd "C:\Users\Minh Vu\Desktop\Spring 2026\eecs582\eecs-582-capstone\frontend"
npm run dev -- --host
```

## Troubleshooting

### `.venv\Scripts\python.exe` does not exist

Your virtual environment was probably created from Git Bash or `MINGW64`.

Fix it by recreating the venv from PowerShell:

```powershell
Remove-Item -Recurse -Force .venv
py -3.12 -m venv .venv
```

### `py -3.12 -m venv .venv` fails

Python `3.12` is probably not installed or not registered with the Windows launcher.

Check available versions:

```powershell
py -0
```

### The site says the ML service is offline

First check whether the ML service is actually healthy:

```powershell
Invoke-RestMethod http://127.0.0.1:8000/health
```

If that returns `status = ok`, the ML service is running.

If the ingest logs show timeouts when posting to ML, restart with `.\dev-stack.ps1` so ingest gets the local `ML_HTTP_TIMEOUT=5s` setting from the Windows launcher.

### The frontend works but there is no data

Check these endpoints:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/api/v1/ping
Invoke-RestMethod http://127.0.0.1:8000/health
```

If both return OK, check the ingest window for runtime errors.
