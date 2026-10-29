# Publishing a handheld version (server owner)

1. Download `birdROMs.muxapp` from the GitHub Release for the version, tagged like `v2026.10.29.1`.
2. Publish it on the server (in the server repo's `backend/`, or inside the backend container):

   ```
   uv run python manage.py publish_handheld /path/to/birdROMs.muxapp
   ```

   The website then offers it on the **Handheld** page and adds `server.json` (the server address) to each
   download. Details: `docs/handheld-app.md` in the server repo.
3. Handhelds that are already paired pick up the published package on their next launch; new friends install it from the website ([install.md](install.md)).

Only if the release does not exist, build it yourself: `just test && just package` (see the README), then
publish `dist/birdROMs.muxapp` as above.
