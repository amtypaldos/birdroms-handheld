# Publishing a handheld version (server owner)

The handheld app and the server are released on their own. Both use date tags
(`vYYYY.MM.DD`, or `vYYYY.MM.DD.N` the same day). The server image bundles the release
named in the server repo's `handheld-version` file, not the server's own tag.

1. Tag this repo (`birdROMs handheld YYYY.MM.DD`). The GitHub Release contains `birdROMs.muxapp`.
2. In the server repo, set `handheld-version` to that tag. Tag the server
   (`birdROMs YYYY.MM.DD`) when the image should ship. A site-only server release leaves
   the pin unchanged. The image build downloads the pinned file and the container installs
   it on startup. You do not copy it yourself.
3. To publish by hand instead (in the server repo's `backend/`, or inside the backend container):

   ```
   uv run python manage.py publish_handheld /path/to/birdROMs.muxapp
   ```

   The website then offers it on the **Handheld** page and adds `server.json` (the server address) to each
   download. Details: `docs/handheld-app.md` in the server repo.
4. Handhelds that are already paired pick up the published package on their next launch; new friends install it from the website ([install.md](install.md)).

Only if the release does not exist, build it yourself: `just test && just package` (see the README), then
publish `dist/birdROMs.muxapp` as above.
