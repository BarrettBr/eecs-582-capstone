# eecs-582-capstone

Note to Grader: School documents are in `docs/School`.

An AI-assisted industrial process monitoring and control demonstrator.

## Quick Tour

- `docs/` Project notes and documentation. See `docs/School` for course documents.
- `ingest/` Data ingestion pipeline and SQL schema/migrations (View `docs/ingest-sql.md` for a guide on SQL with Go).
- `ml/` Model training and artifacts.
- `web/` Web app frontend and static assets.

## Open Plc Quick Notes

- This is mainly an unorganized list of things I had to do to get Open PLC working on my system for reference if you are messing with it and experience issues
- Download the Runtime + editor below
- I had to change the install.sh from python3-venv to python3-virtualenv on each line since my package manager had it under a different name
- Setup and edit the board (in the editor) to runtime v4 as it defaults to 3 additionally enter the ip `127.0.0.1` and click connect to connect your editor to the runtime
- Below is some basic "Hello world" code to test your open plc on (Program section)

```plc
(* Increment once per scan *)
counter := counter + 1;

(* Wrap so it stays in a small range *)
IF counter >= 1000 THEN
  counter := 0;
END_IF;
```

- Make sure to add above a variable called `counter local UINT %MW0 0` this adds it to the first holding register with initial value 0
- The runtime is basically the device and the editor is a way to edit the code running on it
- Since we communicate over modbus TCP modbus does this with slave / master device stuff so they handle that in the plugins.conf file I had to change the slave from 0,0 to 1,1 then restart my openplc-runtime systemctl service

## Resources

- [Autonomy, OpenPLC downloads](https://autonomylogic.com/runtime)
