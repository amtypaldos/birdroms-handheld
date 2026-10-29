# Publishing a new handheld version (owner)

Friends install birdROMs from the **Handheld** page of the birdROMs website (see [install.md](install.md)).
The server keeps one file, `birdROMs.muxapp`, and adds a `server.json` naming itself when someone
downloads it. That file is how a brand-new install knows which server to pair with (the handheld has no
keyboard). It is separate from `config.json`, which holds the pairing token, so updates never log anyone
out.

## Steps

1. Build the package on the Mac (needs `zig`, `go`, `just` and the git-ignored `sysroot/`, see the README):

   ```
   just test
   just package          # writes dist/birdROMs.muxapp
   ```

2. Publish it to the server. In the server repo (or inside the backend container):

   ```
   cd backend
   uv run python manage.py publish_handheld /path/to/dist/birdROMs.muxapp
   ```

   With Docker, copy the file into the container first, then run the same command there. It checks the
   package is rooted at `birdROMs/` and has the app and launcher, then swaps the file in atomically.
   The file ends up at `HANDHELD_APP_PATH` (default `MEDIA_ROOT/handheld/birdROMs.muxapp`).
   Server-side details: `docs/handheld-app.md` in the server repo.

3. Tell friends to download the new file from the Handheld page and reinstall it (install.md, "Updating").

Open the site at the address friends use (your public https address) when testing a download, because the
server bakes the address from the request into `server.json`.

## For a single device (your own)

`just deploy` copies the binary straight onto a device over SSH. Friends use the Handheld page instead.
