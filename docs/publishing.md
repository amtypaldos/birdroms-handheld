# Publishing a handheld version (server owner)

1. The GitHub Release for the version (tagged like `v2026.10.29.2`) contains `birdROMs.muxapp`.
2. Tag the **server** repo with the same name. Its image build downloads that file and the
   container installs it on startup. You do not copy it yourself.
3. To publish by hand instead (in the server repo's `backend/`, or inside the backend container):

   ```
   uv run python manage.py publish_handheld /path/to/birdROMs.muxapp
   ```

   The website then offers it on the **Handheld** page and adds `server.json` (the server address) to each
   download. Details: `docs/handheld-app.md` in the server repo.
4. Handhelds that are already paired pick up the published package on their next launch; new friends install it from the website ([install.md](install.md)).

Only if the release does not exist, build it yourself: `just test && just package` (see the README), then
publish `dist/birdROMs.muxapp` as above.
