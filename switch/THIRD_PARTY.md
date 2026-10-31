# Bundled components

- nlohmann/json 3.12.0: include/nlohmann/json.hpp, MIT license in adjacent LICENSE.MIT.
- Nayuki QR Code generator 1.8.0: include/platform/qrcodegen.h and
  source/platform/qrcodegen.c, MIT license embedded in both files.
- Chakra Petch Bold: copied from existing handheld assets, SIL Open Font License
  in assets/OFL.txt.
- Homebrew Menu icon is derived from the existing package/glyph/birdroms.svg.
- devkitPro official example Makefile adapted for this build. Runtime libraries
  (libnx, SDL2, SDL2_image, SDL2_ttf, curl and their dependencies) come from the
  official devkitPro image pinned in Dockerfile, rather than vendored source.
