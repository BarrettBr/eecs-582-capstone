# OpenPLC Runtime v4 Reference

This document captures setup decisions and issues encountered while integrating OpenPLC Runtime v4 with this project
A lot of the file locations below are listed based on linux, however it should be very similar in windows just look for it's install location and lok at relative pathing

## Framework

- Runtime target: OpenPLC Runtime v4 on Linux
- Transport used by ingest: Modbus TCP

## Quick Setup Checklist

1. Install OpenPLC Runtime + Editor from the official download page.
2. If install fails due to package naming, replace `python3-venv` with `python3-virtualenv` in the installer.
3. In OpenPLC Editor, set board/target to Runtime v4 (it can default to v3).
4. Set runtime IP (local setup example: `127.0.0.1`).
5. In OpenPLC Editor, add a Modbus TCP `Server`, set port to `1502`, and enable it.
6. Compile and upload/deploy from the folder/upload icon. Compile alone is not enough it will break.
7. If server settings are changed, recompile and upload again so runtime gets the update.
8. Verify runtime service is running:
   - `sudo systemctl status openplc-runtime.service`
9. Enable and verify Modbus TCP plugin (details below).
10. Confirm listening port is actually working:
    - `ss -ltnp | grep -E ':1502|:502'`

## Runtime and Service Notes

- On Linux, runtime runs as a systemd service.
- Service name:
  - `openplc-runtime.service`
- After config changes, restart runtime:
  - `sudo systemctl restart openplc-runtime.service`

## Modbus TCP Plugin

OpenPLC Runtime v4 uses plugins, and Modbus TCP could be disabled by default depending on your OS.
This is used to actually transport the data over to our ingestion service so it is needed to recieve it on our side.

1. Open plugin config:
   - `/opt/openplc-runtime/plugins.conf`
2. Find the `modbus_slave` line (Should be first).
3. Ensure enabled flag is set to `1` (example pattern: `modbus_slave,...,1,0,...`).
4. Restart runtime service after editing.
5. Confirm runtime is listening on Modbus port (`1502` or `502`, depending on config/runtime mode, for this I defaulted it to 1502 for less overlap).

## Register Mapping Notes

- A variable declared at `%MW0` in OpenPLC Runtime v4 did not appear at Modbus holding register `0` in this setup.
- Offset: `%MW0` was at address `1024`
- This offset is why ingest environment/config values use `1024` for the holding register.

If polling returns zeros or out-of-range errors, check both:

- Configured register address in ingest
- OpenPLC address offset for your runtime version/config matches properly

## Example Program for testing

```st
VAR
  counter AT %MW0 : UINT;
END_VAR

counter := counter + 1;
```

## Common Failures I experienced

- Compile succeeded but values never update in runtime:
  - Program was compiled but not uploaded/deployed.
- Modbus still unavailable after compile/upload:
  - Modbus `Server` in Editor was not added/enabled on port `1502`, or settings were changed without re-uploading.
- Ingest cannot connect:
  - Modbus TCP plugin disabled in `plugins.conf`.
- Modbus port not open:
  - Service not running or not restarted after config changes.
- Ingest connects but reads wrong values (Mine kept reading 0 on repeat):
  - Register offset mismatch (`%MW0` vs Modbus address base).

## Troubleshooting

1. Confirm runtime process/service is active.
2. Confirm Modbus plugin enabled in `plugins.conf`.
3. Restart runtime service.
4. Confirm port listener with `ss`.
5. In OpenPLC Editor, confirm Modbus `Server` is enabled and set to `1502`.
6. Re-upload PLC program from editor (not compile-only).
7. Verify register address configured in ingest (including offset).

## References

- OpenPLC Runtime and Editor downloads: <https://autonomylogic.com/runtime>
